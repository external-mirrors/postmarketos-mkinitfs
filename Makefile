.POSIX:
.SUFFIXES: .1 .1.scd

VERSION?=$(shell git describe --tags --dirty 2>/dev/null || echo 0.0.0)
VPATH=doc
VENDORED="mkinitfs-vendor-$(VERSION)"
PREFIX?=/usr/local
BINDIR?=$(PREFIX)/sbin
MANDIR?=$(PREFIX)/share/man
SHAREDIR?=$(PREFIX)/share
GO?=go
GOFLAGS?=
LDFLAGS+=-s -w -X main.Version=$(VERSION)
RM?=rm -f
GOTESTOPTS?=-count=1 -race
GOTEST?=go test ./...
DISABLE_GOGC?=

ifeq ($(DISABLE_GOGC),1)
	LDFLAGS+=-X main.DisableGC=true
endif

GOSRC!=find * -name '*.go'
GOSRC+=go.mod go.sum

DOCS := \
	mkinitfs.1

all: mkinitfs $(DOCS)

mkinitfs: $(GOSRC)
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o mkinitfs ./cmd/mkinitfs

.1.scd.1:
	scdoc < $< > $@

doc: $(DOCS)

.PHONY: fmt
fmt:
	gofmt -w .

test:
	@if [ `gofmt -l . | wc -l` -ne 0 ]; then \
		gofmt -d .; \
		echo "ERROR: source files need reformatting with gofmt"; \
		exit 1; \
	fi
	@staticcheck ./...

	$(GOTEST) $(GOTESTOPTS)

clean:
	$(RM) mkinitfs $(DOCS)
	$(RM) $(VENDORED)*

install: $(DOCS) mkinitfs
	install -Dm755 mkinitfs -t $(DESTDIR)$(BINDIR)/
	install -Dm644 mkinitfs.1 -t $(DESTDIR)$(MANDIR)/man1/

.PHONY: checkinstall
checkinstall:
	test -e $(DESTDIR)$(BINDIR)/mkinitfs
	test -e $(DESTDIR)$(MANDIR)/man1/mkinitfs.1

RMDIR_IF_EMPTY:=sh -c '! [ -d $$0 ] || ls -1qA $$0 | grep -q . || rmdir $$0'

vendor:
	go mod vendor
	tar czf $(VENDORED).tar.gz vendor/
	sha512sum $(VENDORED).tar.gz > $(VENDORED).tar.gz.sha512
	$(RM) -rf vendor

uninstall:
	$(RM) $(DESTDIR)$(BINDIR)/mkinitfs
	${RMDIR_IF_EMPTY} $(DESTDIR)$(BINDIR)
	$(RM) $(DESTDIR)$(MANDIR)/man1/mkinitfs.1
	$(RMDIR_IF_EMPTY) $(DESTDIR)$(MANDIR)/man1

.PHONY: all clean install uninstall test vendor
