# shall

shall keeps a product's mission, needs and requirements as reviewed text in
git, so that people and agents can check them, browse them and discuss changes
to them in one place.

A set of requirements is plain YAML next to the code: why the product exists,
who it serves and what they need, and the requirements that answer those
needs, each with a stable id. `shall check` verifies that a set follows the
format, the way a compiler or a linter verifies code, and runs as a
pre-commit hook.

The format is defined in [doc/format.md](doc/format.md), with the shape of
each file also given as JSON Schema in [schema/format-1/](schema/format-1/).

## Running it on this repository

This repository is itself a set: [shall.yaml](shall.yaml) at the root points
to shall's own mission and requirements in `doc/requirements/`. With Go 1.27
or newer, from the root of a checkout:

```bash
go run ./cmd/shall check
```

```text
doc/requirements/baseline.shall.yaml:4: warning: R7 serves: the requirement serves nothing
doc/requirements/baseline.shall.yaml:12: warning: R8 serves: the requirement serves nothing
…
0 errors, 12 warnings
```

Each finding names the file, line, item and field it concerns. Warnings do
not fail the run; errors do. The twelve warnings are real: the baseline
requirements serve no need yet.

To install the binary instead, which puts `shall` in `$(go env GOPATH)/bin`:

```bash
go install ./cmd/shall
```

## Usage

```text
shall check [-json] [-warnings-as-errors] [directory]
```

`shall check` checks every set whose manifest, a file named `shall.yaml`, lies
under the directory, the current one by default.

| Option | Effect |
|---|---|
| `-json` | prints the findings as one JSON document, for scripts and agents |
| `-warnings-as-errors` | fails the run on warnings too |

The exit status is 0 when nothing failed, 1 when the check found errors, and
2 when the command was used wrongly.

```text
shall diff [-json] [-base ref] [-head ref] [directory]
```

`shall diff` compares the sets under the directory between two git refs,
`HEAD` and the working tree by default, item by item: what was added,
removed, retired or changed, field by field with the changed words marked
`[-old-]{+new+}`, and the checker's findings gained and lost. It exits with 0
when nothing changed, 1 when something did and 2 when it cannot compare;
`-json` prints the same for scripts, as [doc/diff.md](doc/diff.md) describes.

```text
shall serve [-addr host:port] [directory]
```

`shall serve` shows each set under the directory as one document in the
browser, at http://127.0.0.1:7180/ unless `-addr` gives another address.
Chapters unfold as the reader reaches them, an outline beside the document
leads to each, every id is a link, the checker's findings sit on the items
they concern, and the page follows the files as they are saved. Unless given
an address, it accepts connections only from this machine.

The page is built from a JSON API that scripts and agents can read directly;
[doc/api.md](doc/api.md) describes it.

## As a pre-commit hook

In the `.pre-commit-config.yaml` of the repository holding the set:

```yaml
repos:
  - repo: https://github.com/koniarik/shall
    rev: <tag or commit>
    hooks:
      - id: shall-check
```

pre-commit builds the hook from source and fetches Go when the machine has
none.

## A minimal set

```yaml
# shall.yaml
name: coffee
format: 1
path: requirements
```

```yaml
# requirements/mission.shall.yaml
title: Why The Machine Exists
mission: Coffee in the office without anyone looking after the machine.
items:
  - id: S1
    title: Office User
    text: Wants coffee without having to look after the machine.
    children:
      - id: N1.1
        title: Know Before Brewing
        text: Know before I brew that the tank needs water.
        serves: [mission]
        status: hypothesis
```

```yaml
# requirements/tank.shall.yaml
title: Water Tank
items:
  - id: R2
    title: Refill Lamp
    text: When the tank becomes empty, the machine SHALL light the refill lamp.
    serves: [N1.1]
    status: approved
```

[doc/format.md](doc/format.md) explains every kind of item and field, with a
good and a bad example of each.

## Licence

shall is under the MIT licence in [LICENSE](LICENSE). The libraries it is
built from, and their licences, are listed in [doc/licences.md](doc/licences.md).
