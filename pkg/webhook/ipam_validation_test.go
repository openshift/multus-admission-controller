package webhook

import (
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	netv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
)

func whereaboutsConfig(ipam map[string]interface{}) string {
	config := map[string]interface{}{
		"cniVersion": "0.3.1",
		"name":       "whereabouts-test",
		"type":       "macvlan",
		"mode":       "bridge",
		"ipam":       ipam,
	}
	bytes, err := json.Marshal(config)
	Expect(err).NotTo(HaveOccurred())
	return string(bytes)
}

func whereaboutsNAD(name string, ipam map[string]interface{}) netv1.NetworkAttachmentDefinition {
	return netv1.NetworkAttachmentDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: netv1.NetworkAttachmentDefinitionSpec{
			Config: whereaboutsConfig(ipam),
		},
	}
}

var _ = Describe("Whereabouts IPAM validation", func() {
	DescribeTable("validateWhereaboutsRange rejects invalid values",
		func(rangeStr string) {
			err := validateWhereaboutsRange(rangeStr)
			Expect(err).To(HaveOccurred())
		},
		Entry("original repro range", "abc.169.1.0/24"),
		Entry("original repro exclude style", "a.b.c.d/23"),
		Entry("plain text", "abcd"),
		Entry("placeholder octets", "x.x.x.x/24"),
		Entry("out of range octets", "999.999.999.999/24"),
		Entry("missing prefix", "192.168.1.0"),
		Entry("invalid prefix", "192.168.1.0/33"),
		Entry("empty string with slash", "/24"),
		Entry("only slash", "/"),
		Entry("hostname not cidr", "example.com/24"),
		Entry("invalid hyphen range start", "abcd-192.168.1.0/24"),
		Entry("invalid hyphen range end", "192.168.1.10-abcd/24"),
	)

	DescribeTable("validateWhereaboutsRange accepts valid values",
		func(rangeStr string) {
			err := validateWhereaboutsRange(rangeStr)
			Expect(err).NotTo(HaveOccurred())
		},
		Entry("ipv4 cidr", "192.168.169.0/24"),
		Entry("ipv4 host route", "192.168.169.10/32"),
		Entry("ipv6 cidr", "2001:db8::/64"),
		Entry("whereabouts hyphen range", "192.168.1.10-192.168.1.20/24"),
	)

	DescribeTable("validateIPAMConfigs rejects invalid whereabouts ipam",
		func(ipam map[string]interface{}) {
			err := validateIPAMConfigs([]byte(whereaboutsConfig(ipam)))
			Expect(err).To(HaveOccurred())
		},
		Entry("invalid range only", map[string]interface{}{
			"type":  "whereabouts",
			"range": "abc.169.1.0/24",
		}),
		Entry("valid range invalid exclude", map[string]interface{}{
			"type":  "whereabouts",
			"range": "192.168.169.0/24",
			"exclude": []interface{}{
				"a.b.c.d/23",
			},
		}),
		Entry("invalid gateway", map[string]interface{}{
			"type":    "whereabouts",
			"range":   "192.168.169.0/24",
			"gateway": "abcd",
		}),
		Entry("invalid range_start", map[string]interface{}{
			"type":        "whereabouts",
			"range":       "192.168.169.0/24",
			"range_start": "not-an-ip",
		}),
		Entry("invalid range_end", map[string]interface{}{
			"type":      "whereabouts",
			"range":     "192.168.169.0/24",
			"range_end": "xyz",
		}),
		Entry("invalid ipRanges range", map[string]interface{}{
			"type": "whereabouts",
			"ipRanges": []interface{}{
				map[string]interface{}{
					"range": "abcd/24",
				},
			},
		}),
		Entry("invalid ipRanges exclude", map[string]interface{}{
			"type": "whereabouts",
			"ipRanges": []interface{}{
				map[string]interface{}{
					"range": "192.168.10.0/24",
					"exclude": []interface{}{
						"foo.bar.baz/32",
					},
				},
			},
		}),
		Entry("multiple excludes with one invalid", map[string]interface{}{
			"type":  "whereabouts",
			"range": "192.168.169.0/24",
			"exclude": []interface{}{
				"192.168.169.10/32",
				"abcd/24",
			},
		}),
	)

	DescribeTable("validateIPAMConfigs accepts valid whereabouts ipam",
		func(ipam map[string]interface{}) {
			err := validateIPAMConfigs([]byte(whereaboutsConfig(ipam)))
			Expect(err).NotTo(HaveOccurred())
		},
		Entry("basic valid config", map[string]interface{}{
			"type":  "whereabouts",
			"range": "192.168.169.0/24",
			"exclude": []interface{}{
				"192.168.169.10/32",
			},
			"gateway": "192.168.169.1",
		}),
		Entry("ipv6 config", map[string]interface{}{
			"type":  "whereabouts",
			"range": "2001:db8::/64",
			"exclude": []interface{}{
				"2001:db8::1/128",
			},
		}),
		Entry("ipRanges config", map[string]interface{}{
			"type": "whereabouts",
			"ipRanges": []interface{}{
				map[string]interface{}{
					"range": "192.168.20.0/24",
					"exclude": []interface{}{
						"192.168.20.1/32",
					},
				},
			},
		}),
		Entry("hyphen range", map[string]interface{}{
			"type":  "whereabouts",
			"range": "192.168.50.10-192.168.50.20/24",
		}),
	)

	It("does not validate non-whereabouts ipam plugins", func() {
		config := `{
			"cniVersion": "0.3.1",
			"type": "macvlan",
			"ipam": {
				"type": "host-local",
				"subnet": "abcd/24"
			}
		}`
		err := validateIPAMConfigs([]byte(config))
		Expect(err).NotTo(HaveOccurred())
	})

	It("validates whereabouts ipam inside a plugin list", func() {
		config := `{
			"cniVersion": "0.3.1",
			"name": "conflist-network",
			"plugins": [{
				"type": "macvlan",
				"ipam": {
					"type": "whereabouts",
					"range": "abcd/24"
				}
			}]
		}`
		err := validateIPAMConfigs([]byte(config))
		Expect(err).To(HaveOccurred())
	})

	DescribeTable("validateNetworkAttachmentDefinition end-to-end whereabouts checks",
		func(nad netv1.NetworkAttachmentDefinition, shouldFail bool) {
			allowed, err := validateNetworkAttachmentDefinition(nad)
			if shouldFail {
				Expect(allowed).To(BeFalse())
				Expect(err).To(HaveOccurred())
				return
			}
			Expect(allowed).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		},
		Entry(
			"original bug repro",
			whereaboutsNAD("checking-nad", map[string]interface{}{
				"type":  "whereabouts",
				"range": "abc.169.1.0/24",
				"exclude": []interface{}{
					"a.b.c.d/23",
				},
			}),
			true,
		),
		Entry(
			"invalid range abcd",
			whereaboutsNAD("bad-range", map[string]interface{}{
				"type":  "whereabouts",
				"range": "abcd",
			}),
			true,
		),
		Entry(
			"valid whereabouts nad",
			whereaboutsNAD("valid-whereabouts-nad", map[string]interface{}{
				"type":  "whereabouts",
				"range": "192.168.169.0/24",
				"exclude": []interface{}{
					"192.168.169.10/32",
				},
				"gateway": "192.168.169.1",
			}),
			false,
		),
	)

	DescribeTable("validateNetworkAttachmentDefinition rejects many invalid ranges",
		func(rangeValue string) {
			nad := whereaboutsNAD("invalid-range-nad", map[string]interface{}{
				"type":  "whereabouts",
				"range": rangeValue,
			})
			allowed, err := validateNetworkAttachmentDefinition(nad)
			Expect(allowed).To(BeFalse())
			Expect(err).To(HaveOccurred())
		},
		Entry("abcd", "abcd"),
		Entry("abc.def.ghi.jkl/24", "abc.def.ghi.jkl/24"),
		Entry("a.b.c.d/23", "a.b.c.d/23"),
		Entry("192.168.1.256/24", "192.168.1.256/24"),
		Entry("not-an-ip/24", "not-an-ip/24"),
	)
})

var _ = Describe("Whereabouts IPAM validation helpers", func() {
	It("formats NAD config for table tests", func() {
		config := whereaboutsConfig(map[string]interface{}{
			"type":  "whereabouts",
			"range": "192.168.1.0/24",
		})
		Expect(config).To(ContainSubstring(`"type":"whereabouts"`))
		Expect(config).To(ContainSubstring(fmt.Sprintf(`"range":"192.168.1.0/24"`)))
	})
})
