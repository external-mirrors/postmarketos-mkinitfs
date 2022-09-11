`postmarketos-mkinitfs` is a tool for generating an initramfs (and installing
it) on postmarketOS.

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

The application uses configuration from `/etc/deviceinfo`, and does not support
any other options at runtime. It can be run simply by executing:

```
$ postmarketos-mkinitfs
```

For historical reasons, a symlink from `mkinitfs` to `postmarketos-mkinitfs` is
also installed by the makefile's `install` target.
