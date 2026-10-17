# shall format 1

A **set** states why one product exists and what it promises: the results it
aims for, who it serves, what they need, and the requirements that answer
those needs. A set is a manifest and the files it points to, kept in git like
code.

Three conventions hold throughout. A field is required unless marked
optional. A field that names other items holds a list of ids, except
`stakeholder`, which holds one. A field this document does not define is an
error.

## Manifest

A file named `shall.yaml`, anywhere in a repository. It says where a set lives
and how to read it.

| Field | Meaning |
|---|---|
| `name` | The set's name, shown wherever the set is listed. |
| `format` | The format version the set's files are written in: the number `1`. |
| `path` | Optional. The directory holding the set's files, relative to the manifest. When absent, the manifest's own directory. |
| `depends` | Optional. The sets this one builds on, each under the name this set uses for it: lowercase letters, digits and hyphens, starting with a letter. |

Each dependency says where the other set's manifest is:

| Field | Meaning |
|---|---|
| `git` | Optional. The repository holding it. When absent, the same repository as this set, at the same revision. |
| `path` | Optional. The directory of its manifest, relative to the root of that repository. When absent, the repository's root. |
| `rev` | The revision to read it at: a tag or a commit, so that the set always reads the same. Naming a branch is a warning, since a branch moves. Required with `git`. |

```yaml
depends:
  company:
    git: https://github.com/acme/standards
    rev: v2
  department:
    path: firmware/standards
```

## Files

Every file under `path`, in any subdirectory, whose name ends in `.shall.yaml`
belongs to the set.

**The mission file**, `mission.shall.yaml`, exists exactly once in every set,
a standard that products depend on included, and holds why the product
exists.

| Field | Meaning |
|---|---|
| `mission` | One statement of why the product exists. |
| `ranking` | Optional. Objective ids, highest first. When two objectives conflict, the one listed earlier wins; objectives not listed are not ranked. |
| `objectives`, `stakeholders`, `needs` | Optional. The items of those kinds, which appear in no other file. |
| `retired` | Optional. Retired top-level items of those kinds. |

**Chapter files** are every other file. They hold the requirements, which
read in the order they are written.

| Field | Meaning |
|---|---|
| `title` | The chapter's title. |
| `requirements` | The chapter's top-level requirements. |
| `retired` | Optional. Retired top-level requirements of this chapter. |

## Ids

An id is its kind's prefix followed by a number, optionally followed by `.`
and a number, repeated: `O3`, `N3.2`, `R12.1.4`. Numbers start at 1, have no
leading zeros, and need not be consecutive. A top-level id has no `.`; a
child's id is its parent's id followed by `.` and a number.

Ids are unique within a set and never reused. An id does not depend on the
file its item lives in, so a requirement can move between chapters unchanged.
Moving a child to another parent, or to the top level, changes its id: the
old id is retired and the item takes a new one.

## Retiring an item

An item that is no longer wanted is retired rather than deleted without
trace. Its entry goes into the `retired` list of the place that held it: a
file for a top-level item, its parent item for a child. An entry has the
item's `id` and the `title` it had; git holds the rest of its history.

Retiring an item retires its children with it. A retired id never returns
anywhere in the set. A reference to a retired id is a warning, so that a large
change can land in several steps.

## Items

Every item has these fields:

| Field | Meaning |
|---|---|
| `id` | Its id, with its kind's prefix. |
| `title` | A short name, shown wherever the item is listed. |
| `text` | What the item says. Each kind below gives `text` its meaning. |
| `children` | Optional. Items of the same kind that refine this one. |
| `mirrors` | Optional. An item of the same kind in a set this one depends on, named `name/id`, that this item stands for here. |
| `notes` | Optional. A list of free-text notes for whoever reads the item. Nothing checks or interprets them. |
| `links` | Optional. A list of links to things outside the set, such as the evidence that a need is validated or a requirement verified. |
| `retired` | Optional. Retired children of this item. |

A link names its kind by its key, and may carry a `title`. Format 1 has one
kind, `url`:

```yaml
links:
  - url: https://ci.example.com/runs/4711
    title: Tank tests on hardware revision C
```

**Mirrors.** Every item of every set this one depends on applies here too,
and each has exactly one mirror in this set. An item without a mirror is a
warning, unless the mirror of its parent is waived; two mirrors of the same
item are an error. Only requirements can be waived: the objectives,
stakeholders and needs of a set this one depends on always apply. A mirror's `title` is the title of the item it mirrors,
and a different title is a warning. Its `text` and its other fields say what
the item means in this set's own context.

A child takes over the fields of its parent that its kind lists as
inherited, unless it sets them itself; a value the child sets replaces the
inherited one whole. `id`, `title`, `text`, `children`, `status`, `mirrors`,
`notes`, `links` and `retired` are never inherited, since they describe the
item itself.

### Objective — `objectives`, prefix `O`

A result the product exists to achieve. Objectives are what needs are judged
against, and the ranking settles conflicts between them.

| Field | Meaning |
|---|---|
| `text` | The result, stated so it reads as true once achieved. |
| `measure` | What to observe to tell how far the objective is met. |

Inherited by children: `measure`.

```yaml
# good: a result that can be observed, and a way to observe it
- id: O1
  title: No Dry Brews
  text: No brew starts while the water tank is empty.
  measure: brews started on an empty tank, per thousand brews.

# bad: no reader can tell when it is met, and nothing measures it
- id: O2
  title: Better Machine
  text: The machine is better.
```

### Stakeholder class — `stakeholders`, prefix `S`

A role whose interest the product serves, such as a tester or an operator. A
class is a role, not a person.

| Field | Meaning |
|---|---|
| `text` | The class's interest in the product. |

Inherited by children: nothing.

```yaml
# good: a role, and what it wants from the product
- id: S1
  title: Office User
  text: Wants coffee without having to look after the machine.

# bad: a person, whose interest goes when they leave
- id: S2
  title: John From Facilities
  text: Asked for the machine.
```

### Need — `needs`, prefix `N`

Something one stakeholder class needs from the product, in that class's own
terms. Needs are where requirements come from.

| Field | Meaning |
|---|---|
| `text` | The need, as the class would say it. |
| `stakeholder` | The class that has it. |
| `serves` | The objectives it advances. The list may also hold the word `mission`, for a need that serves the mission directly. |
| `status` | `hypothesis` until real use confirms it, then `validated`. A need found to be wrong is retired. |

Inherited by children: `stakeholder`, `serves`.

```yaml
# good: the user's words, with no solution in them
- id: N1
  title: Know Before Brewing
  stakeholder: S1
  text: Know before I brew that the tank needs water.
  serves: [O1]
  status: hypothesis

# bad: a design in the system's words; the user never asked for a float
- id: N2
  title: Float Sensor
  stakeholder: S1
  text: The tank has a float sensor.
  serves: [O1]
  status: hypothesis
```

### Requirement — `requirements`, prefix `R`

One promise the product keeps, specific enough to verify.

| Field | Meaning |
|---|---|
| `text` | The promise, as one sentence. |
| `serves` | The needs or requirements it exists for, in this set or another. Leaving it out is a warning, so that a requirement can be written before its reason is placed. |
| `rationale` | Optional. Why the promise exists, including options considered and not taken. Requirements that rest on the same reason serve the requirement that states it. |
| `verify` | Optional. How the promise is shown to hold: `test` (the default), `inspection`, `analysis` or `demonstration`. |
| `status` | Where the promise stands: `draft` is written but not yet agreed; `approved` is agreed as a promise the product will keep; `implemented` means the product does it; `verified` means it has been shown to hold by its `verify` method; `waived` means it does not apply to this product. |

Inherited by children: `serves`, `rationale`, `verify`.

An item's **effective status** is the lowest of its own `status` and the
effective status of its children, in the order `draft`, `approved`,
`implemented`, `verified`. A requirement written as `verified` with one child
still `implemented` is effectively `implemented`; both are shown, and the
difference is not a fault. A `waived` requirement is effectively waived
together with all its children, which need no status of their own, and a
waived child does not lower its parent.

**Keywords.** RFC 2119 defines words that carry obligation when written in
capitals: MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, SHOULD NOT,
RECOMMENDED, NOT RECOMMENDED, MAY and OPTIONAL. A requirement's text uses
SHALL, or SHALL NOT, exactly once, and none of the others: a promise either
holds or it is not a requirement. A keyword counts only as a whole word in
capitals, so SHALL NOT counts once and SHALLOW not at all. Written in
lowercase, these words carry no obligation, which is how a requirement can
talk about them.

**Sentence forms.** A requirement takes one of five forms, named by the EARS
method. A requirement in none of them is a warning.

| Form | Shape | Example |
|---|---|---|
| ubiquitous | The *system* SHALL *response*. | The machine SHALL keep a log of every brew. |
| event | When *trigger*, the *system* SHALL *response*. | When the tank becomes empty, the machine SHALL light the refill lamp. |
| state | While *state*, the *system* SHALL *response*. | While a brew is running, the machine SHALL keep the lid locked. |
| unwanted | If *fault*, then the *system* SHALL *response*. | If the level sensor fails, then the machine SHALL refuse to brew. |
| optional | Where *feature*, the *system* SHALL *response*. | Where a display is fitted, the machine SHALL show the water level. |

```yaml
# good: one condition, one promise, checkable by a test
- id: R1
  title: Refill Lamp
  text: When the tank becomes empty, the machine SHALL light the refill lamp.
  serves: [N1]
  status: approved

# bad: "quickly" cannot be tested, and it makes two promises in one
- id: R2
  title: Water Handling
  text: The machine SHALL detect low water quickly and alert the user.
  serves: [N1]
  status: draft
```

## References

| Field | On | Names |
|---|---|---|
| `serves` | need | objectives, and the word `mission` |
| `stakeholder` | need | one stakeholder class |
| `serves` | requirement | needs or requirements |
| `mirrors` | any item | one item of the same kind in a set this one depends on |

An item of another set is named `name/id`, where `name` is the name the
manifest gives that set under `depends`. A standard that every product of a
company or a department meets is a set of its own, which each product depends
on and mirrors.

## Errors and warnings

The rules are what can be checked from the files: fields and their values,
ids, references, and the keywords in a requirement's text. Breaking a rule is
an error, unless the rule says it is a warning, and a checker can treat
warnings as errors. What the descriptions and examples ask of an item's
wording, such as a need being in its class's own terms, guides authors and
reviewers; nothing checks it.

## Example

```yaml
# shall.yaml
name: coffee
format: 1
path: requirements
```

```yaml
# requirements/mission.shall.yaml
mission: Coffee in the office without anyone looking after the machine.
ranking: [O1]
objectives:
  - id: O1
    title: No Dry Brews
    text: No brew starts while the water tank is empty.
    measure: brews started on an empty tank, per thousand brews.
stakeholders:
  - id: S1
    title: Office User
    text: Wants coffee without having to look after the machine.
needs:
  - id: N1
    title: Know Before Brewing
    stakeholder: S1
    text: Know before I brew that the tank needs water.
    serves: [O1]
    status: validated
    notes:
      - Heard from three users in the first week of the trial.
```

```yaml
# requirements/01-tank.shall.yaml
title: Water Tank
requirements:
  - id: R1
    title: Refill Lamp
    text: When the tank becomes empty, the machine SHALL light the refill lamp.
    serves: [N1]
    status: implemented
    links:
      - url: https://ci.example.com/runs/4711
        title: Tank tests on hardware revision C
    children:
      - id: R1.1
        title: Lamp Seen From The Door
        text: The machine SHALL show the refill lamp to anyone standing at the
          kitchen door.
        verify: inspection
        status: implemented
    retired:
      - id: R1.2
        title: Refill Beep
retired:
  - id: R2
    title: Water Handling
```

## Schema

[`schema/format-1/`](../schema/format-1/) gives the shape of each file as JSON
Schema, for editors and other tools. It cannot express every rule here, such
as a child's id extending its parent's or a reference resolving; where the two
differ, this document holds.
