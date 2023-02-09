.POSIX:
.SUFFIXES:

PREFIX?=/usr/local
BINDIR?=$(PREFIX)/sbin
SHAREDIR?=$(PREFIX)/share
GO?=go
GOFLAGS?=
LDFLAGS+=-s -w
RM?=rm -f
GOTEST=go test -count=1 -race

GOSRC!=find * -name '*.go'
GOSRC+=go.mod go.sum

all: mkinitfs

mkinitfs: $(GOSRC)
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o mkinitfs ./cmd/mkinitfs

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

	@$(GOTEST) ./...

clean:
	$(RM) mkinitfs 

install: $(DOCS) mkinitfs
	install -Dm755 mkinitfs -t $(DESTDIR)$(BINDIR)/

.PHONY: checkinstall
checkinstall:
	test -e $(DESTDIR)$(BINDIR)/mkinitfs

RMDIR_IF_EMPTY:=sh -c '! [ -d $$0 ] || ls -1qA $$0 | grep -q . || rmdir $$0'

uninstall:
	$(RM) $(DESTDIR)$(BINDIR)/mkinitfs
	${RMDIR_IF_EMPTY} $(DESTDIR)$(BINDIR)

.PHONY: all clean install uninstall test
