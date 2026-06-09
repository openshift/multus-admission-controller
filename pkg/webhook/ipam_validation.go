package webhook

import (
	"encoding/json"
	"fmt"
	"strings"

	netutils "k8s.io/utils/net"
)

const whereaboutsIPAMType = "whereabouts"

func validateIPAMConfigs(config []byte) error {
	var c map[string]interface{}
	if err := json.Unmarshal(config, &c); err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}

	if plugins, ok := c["plugins"].([]interface{}); ok {
		for _, plugin := range plugins {
			pluginConfig, ok := plugin.(map[string]interface{})
			if !ok {
				return fmt.Errorf("invalid plugin config")
			}
			if err := validatePluginIPAM(pluginConfig); err != nil {
				return err
			}
		}
		return nil
	}

	return validatePluginIPAM(c)
}

func validatePluginIPAM(plugin map[string]interface{}) error {
	ipamRaw, ok := plugin["ipam"]
	if !ok {
		return nil
	}

	ipam, ok := ipamRaw.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid ipam config")
	}

	ipamType, _ := ipam["type"].(string)
	if ipamType != whereaboutsIPAMType {
		return nil
	}

	return validateWhereaboutsIPAM(ipam)
}

func validateWhereaboutsIPAM(ipam map[string]interface{}) error {
	if rangeStr, ok := ipam["range"].(string); ok && rangeStr != "" {
		if err := validateWhereaboutsRange(rangeStr); err != nil {
			return fmt.Errorf("invalid whereabouts ipam range: %w", err)
		}
	}

	if err := validateWhereaboutsStringIP(ipam, "range_start"); err != nil {
		return err
	}
	if err := validateWhereaboutsStringIP(ipam, "range_end"); err != nil {
		return err
	}
	if err := validateWhereaboutsStringIP(ipam, "gateway"); err != nil {
		return err
	}

	if err := validateWhereaboutsExcludeList(ipam["exclude"]); err != nil {
		return err
	}

	ipRangesRaw, ok := ipam["ipRanges"].([]interface{})
	if !ok {
		return nil
	}

	for idx, ipRangeRaw := range ipRangesRaw {
		ipRange, ok := ipRangeRaw.(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid whereabouts ipam ipRanges entry at index %d", idx)
		}

		rangeStr, _ := ipRange["range"].(string)
		if rangeStr != "" {
			if err := validateWhereaboutsRange(rangeStr); err != nil {
				return fmt.Errorf("invalid whereabouts ipam ipRanges[%d].range: %w", idx, err)
			}
		}

		if err := validateWhereaboutsExcludeList(ipRange["exclude"]); err != nil {
			return fmt.Errorf("invalid whereabouts ipam ipRanges[%d].exclude: %w", idx, err)
		}
	}

	return nil
}

func validateWhereaboutsExcludeList(excludeRaw interface{}) error {
	excludeList, ok := excludeRaw.([]interface{})
	if !ok || len(excludeList) == 0 {
		return nil
	}

	for idx, excludeEntry := range excludeList {
		excludeStr, ok := excludeEntry.(string)
		if !ok {
			return fmt.Errorf("invalid exclude entry at index %d", idx)
		}
		if err := validateWhereaboutsRange(excludeStr); err != nil {
			return fmt.Errorf("invalid CIDR in exclude list %s: %w", excludeStr, err)
		}
	}

	return nil
}

func validateWhereaboutsStringIP(ipam map[string]interface{}, field string) error {
	value, ok := ipam[field].(string)
	if !ok || value == "" {
		return nil
	}

	if netutils.ParseIPSloppy(value) == nil {
		return fmt.Errorf("invalid whereabouts ipam %s: %s", field, value)
	}

	return nil
}

func validateWhereaboutsRange(rangeStr string) error {
	parts := strings.SplitN(rangeStr, "-", 2)
	if len(parts) == 2 {
		if netutils.ParseIPSloppy(strings.TrimSpace(parts[0])) == nil {
			return fmt.Errorf("invalid range start IP: %s", parts[0])
		}
		if _, _, err := netutils.ParseCIDRSloppy(strings.TrimSpace(parts[1])); err != nil {
			return fmt.Errorf("invalid CIDR '%s': %w", parts[1], err)
		}
		return nil
	}

	if _, _, err := netutils.ParseCIDRSloppy(rangeStr); err != nil {
		return fmt.Errorf("invalid CIDR %s: %w", rangeStr, err)
	}

	return nil
}
