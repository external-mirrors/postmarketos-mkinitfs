mkinitfs(1) "mkinitfs"

# NAME

mkinitfs

# DESCRIPTION

mkinitfs is a simple, generic tool for generating an initramfs, primarily
developed for use in postmarketOS

# CONCEPTS

mkinitfs is designed to generate two archives, "initramfs" and
"initramfs-extra", however it's possible to configure mkinitfs to run without
generating an initramfs-extra archive. mkinitfs is primarily configured through
the placement of files in specific directories detailed below in the
*DIRECTORIES* section. *deviceinfo* files are also used to provide other
configuration options to mkinitfs, these are covered under the *DEVICEINFO*
section below.

mkinitfs does not provide an init script, or any boot-time logic, it's purpose
is purely to generate the archive(s). mkinitfs does call *boot-deploy* after
creating the archive(s), in order to install/deploy them and any other relevant
boot-related items onto the system.

# DEVICEINFO

The canonical deviceinfo "specification" is at
https://wiki.postmarketos.org/wiki/Deviceinfo_reference

mkinitfs reads deviceinfo values from */etc/deviceinfo*. The following variables
are *required* by mkinitfs:

	- deviceinfo_initfs_compression
	- deviceinfo_mesa_driver
	- deviceinfo_modules_initfs
	- deviceinfo_uboot_boardname

It is a design goal to keep the number of required variables from deviceinfo to
a bare minimum, and to require only variables that don't hold lists of things.

# DIRECTORIES

The following directories are used by mkinitfs to generate the initramfs and
initramfs-extra archives. Directories that end in *-extra* indicate directories
that are used for constructing the initramfs-extra archive, while those without
it are for constructing the initramfs archive.

Configuration under */usr/share/mkinitfs* is intended to be managed by
distributions, while configuration under */etc/mkinitfs* is for users to
create/manage. mkinitfs reads configuration from */usr/share/mkinitfs* first, and then from */etc/mkinitfs*. 

## /usr/share/mkinitfs/files, /etc/mkinitfs/files
## /usr/share/mkinitfs/files-extra, /etc/mkinitfs/files-extra

	Files with the *.files* extension are read as a list of
	files/directories. Each line is in the format:

	```
	<source path>:<destination path>
	```

	The source path is the location, at runtime, of the file or directory
	which will be copied to the destination path within the initramfs
	archive. Specifying a destination path, with *:<destination path>* is
	optional. If it is omitted, then the source path will be used as the
	destination path within the archive. The source and destination paths
	are delimited by a *:* (colon.) Destination path is ignored if the source
	path is a glob that returns more than 1 file. This may change in the future.

[[ *Line in .files*
:< Comment
|  */usr/share/bazz*
:  File or directory */usr/share/bazz* would be added to the archive under */usr/share/bazz*
|  */usr/share/bazz:/bazz*
:  File or directory */usr/share/bazz* would be added to the archive under */bazz*
|  */root/something/\**
:  Everything under */root/something* would be added to the archive under */root/something*
|  */etc/foo/\*/bazz:/foo*
:  Anything that matches the glob will be installed under the source path in the archive. For example, */etc/foo/bar/bazz* would be installed at */etc/foo/bar/bazz* in the archive. The destination path is ignored.

	It's possible to overwrite file/directory destinations from
	configuration in */usr/share/mkinitfs* by specifying the same source
	path(s) under the relevant directory in */etc/mkinitfs*, and changing
	the destination path.

## /usr/share/mkinitfs/hooks, /etc/mkinitfs/hooks
## /usr/share/mkinitfs/hooks-extra*, /etc/mkinitfs/hooks-extra

	Any files listed under these directories are copied as-is into the
	relevant archives. Hooks are generally script files, but how they are
	treated in the initramfs is entirely up to whatever init script is run
	there on boot.

	Hooks are installed in the initramfs under the */hooks* directory, and
	under */hooks-extra* for the initramfs-extra.

## /usr/share/mkinitfs/modules, /etc/mkinitfs/modules
## /usr/share/mkinitfs/modules-extra, /etc/mkinitfs/modules-extra

	Files with the *.modules* extention in these directories are lists of
	kernel modules to include in the initramfs. Individual modules and
	directories can be listed in the files here. Globbing is also supported.

	Modules are installed in the initramfs archive under the same path they
	exist on the system where mkinitfs is executed.

## /usr/share/mkinitfs/dirs, /etc/mkinitfs/dirs

	Files with the *.dirs* extension in these directories are lists of
	directories to create within the initramfs. There is no *-extra* variant,
	since directories are of negligible size.

# AUTHORS

*Clayton Craft* <clayton@craftyguy.net>
