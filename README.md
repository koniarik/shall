# shall

shall keeps a product's mission, needs and requirements as reviewed text in
git, so that people and agents can check them, browse them and discuss changes
to them in one place.

A set of requirements is plain YAML next to the code: why the product exists,
who it serves and what they need, and the requirements that answer those
needs, each with a stable id. `shall check` verifies that a set follows the
format, the way a compiler or a linter verifies code, and runs as a
pre-commit hook.

## shall's own set

This repository is itself a set: [shall.yaml](shall.yaml) at the root points
to shall's own mission and requirements in
[doc/requirements/](doc/requirements/), published from `main` on each push
to [read as a document](https://koniarik.github.io/shall/shall/), as `shall serve` shows them. The
set explains shall as well as specifying it:

- [Format 1](https://koniarik.github.io/shall/shall/T63): every kind of item and field,
  with a good and a bad example of each, and an example set;
- [the command line](https://koniarik.github.io/shall/shall/T64), and
  [installing shall and its pre-commit hook](https://koniarik.github.io/shall/shall/T64.2);
- [what a diff reports](https://koniarik.github.io/shall/shall/T65),
  [the document](https://koniarik.github.io/shall/shall/T66),
  [the API](https://koniarik.github.io/shall/shall/T67) and
  [a static copy](https://koniarik.github.io/shall/shall/T68);
- [how shall is built](https://koniarik.github.io/shall/shall/T69).

## Running it

With Go installed, from the root of a checkout:

```bash
go run ./cmd/shall check
go run ./cmd/shall serve
```

The first checks this repository's set, and the second shows it in the
browser, at the address it prints. `go install ./cmd/shall` puts `shall` in
`$(go env GOPATH)/bin` instead.

## Licence

shall is under the MIT licence in [LICENSE](LICENSE). The libraries it is
built from, and their licences, are listed in [doc/licences.md](doc/licences.md).
