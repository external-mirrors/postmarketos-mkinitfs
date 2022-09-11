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

all: postmarketos-mkinitfs

postmarketos-mkinitfs: $(GOSRC)
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o postmarketos-mkinitfs

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
	$(RM) postmarketos-mkinitfs 

install: $(DOCS) postmarketos-mkinitfs
	install -Dm755 postmarketos-mkinitfs -t $(DESTDIR)$(BINDIR)/
	ln -sf postmarketos-mkinitfs $(DESTDIR)$(BINDIR)/mkinitfs

.PHONY: checkinstall
checkinstall:
	test -e $(DESTDIR)$(BINDIR)/postmarketos-mkinitfs
	test -L $(DESTDIR)$(BINDIR)/mkinitfs

RMDIR_IF_EMPTY:=sh -c '! [ -d $$0 ] || ls -1qA $$0 | grep -q . || rmdir $$0'

uninstall:
	$(RM) $(DESTDIR)$(BINDIR)/postmarketos-mkinitfs
	$(RM) $(DESTDIR)$(BINDIR)/mkinitfs
	${RMDIR_IF_EMPTY} $(DESTDIR)$(BINDIR)

.PHONY: all clean install uninstall test
