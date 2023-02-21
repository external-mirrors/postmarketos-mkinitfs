`mkinitfs` is a tool for generating an initramfs. It was originally designed
for postmarketOS, but a long term design goal is to be as distro-agnostic as
possible. It's capable of generating a split initramfs, in the style used by
postmarketOS, and supports running `boot-deploy` to install/finalize boot files
on a device.

## Building

Building this project requires a Go compiler/toolchain and `make`:

```
$ make
```

To install locally:

```
$ make install
```

Installation prefix can be set in the generally accepted way with setting
`PREFIX`:

```
$ make PREFIX=/some/location
# make PREFIX=/some/location install
```

Other paths can be modified from the command line as well, see the top section of
the `Makefile` for more information.

Tests (functional and linting) can be executed by using the `test` make target:

```
$ make test
```

## Usage

The tool can be run with no options:

```
# mkinitfs
```

Configuration is done through a series of flat text files that list directories
and files, and by placing scripts in specific directories. See `man 1 mkinitfs`
for more information.
