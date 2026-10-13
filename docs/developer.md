# Developer Guide

## Building the admission controller

For now, simply run:

```
make
```

More information to come.

## Vendored packages

We version the vendored packages (which are managed with glide) for scenarios where building cannot download glide packages during build procedures.

When updating vendored package, please keep in mind -- one current work-around is to remove the duplicate inclusion of the `glog` package with:

```
rm -Rf vendor/github.com/intel/multus-cni/vendor/github.com/golang/glog/
```
