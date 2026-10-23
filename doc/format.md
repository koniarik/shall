# shall format 1

A **set** states why one product exists and what it promises: the results it
aims for, who it serves, what they need, and the requirements that answer
those needs, with topics that explain what a reader needs to know to follow
them. A set is a manifest and the files it points to, kept in git like code.

Three conventions hold throughout. A field is required unless marked
optional. A field that names other items holds a list of ids, except
`mirrors`, which holds one. A field this document does not define is an
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
belongs to the set. Each file is a **chapter**, and holds items of any kind,
which read in the order they are written. Chapters read in the order of their
paths, the mission file first.

| Field | Meaning |
|---|---|
| `title` | The chapter's title. |
| `items` | Optional. The chapter's top-level items. |
| `retired` | Optional. Retired top-level items of this chapter. |

**The mission file**, `mission.shall.yaml`, exists exactly once in every set,
a standard that products depend on included. It is a chapter that also holds
why the product exists:

| Field | Meaning |
|---|---|
| `mission` | One statement of why the product exists. |
| `ranking` | Optional. Objective ids, highest first. When two objectives conflict, the one listed earlier wins; objectives not listed are not ranked. |

## Ids

An id is a letter, the prefix of its item's kind, followed by a path of
numbers joined by `.`: `S3`, `N3.2`, `T12.1.4`. The path places the item in
its set's tree. A top-level item's path is one number; a child's path is its
parent's path followed by `.` and a number, whatever the parent's kind.
Numbers start at 1, have no leading zeros, and need not be consecutive.

The path alone names an item. The children of one parent count together,
whatever their kinds, and so do the top-level items of all the set's
chapters, so no two items of a set share a path. A reference whose letter is
not the kind of the item its path names is an error.

An id does not depend on the chapter its item lives in, so a top-level item
can move between chapters unchanged. Moving an item to another parent, or to
or from the top level, changes its id: the old id is retired and the item
takes a new one.

## Retiring an item

An item that is no longer wanted is retired rather than deleted without
trace. Its entry goes into the `retired` list of the place that held it: a
chapter for a top-level item, its parent item for a child. An entry has the
item's `id` and the `title` it had; git holds the rest of its history.

Retiring an item retires its children with it. A retired path never returns,
under any letter. A reference to a retired id is a warning, so that a large
change can land in several steps; a reference to a retired path under a
letter other than the retired item's is an error.

## Items

The items of a set form trees. Every item has these fields:

| Field | Meaning |
|---|---|
| `id` | Its id, with its kind's prefix. |
| `title` | A short name, shown wherever the item is listed. |
| `text` | What the item says. Each kind below gives `text` its meaning; a topic may leave it out. |
| `children` | Optional. The items under this one. |
| `links` | Optional. A list of links to things outside the set, such as the evidence that a need is validated or a requirement verified. |
| `retired` | Optional. Retired children of this item. |

Every item but a topic also has these:

| Field | Meaning |
|---|---|
| `mirrors` | Optional. An item of the same kind in a set this one depends on, named `name/id`, that this item stands for here. |
| `notes` | Optional. A list of free-text notes for whoever reads the item. Nothing checks or interprets them. |

A link names its kind by its key, and may carry a `title`. Format 1 has one
kind, `url`:

```yaml
links:
  - url: https://ci.example.com/runs/4711
    title: Tank tests on hardware revision C
```

**Nesting.** An item may hold items of its own kind, which refine it, and
topics. A topic may hold items of every kind. A stakeholder class may also
hold needs: the needs the class has. No kind holds another otherwise, except
through a topic, under which an item of any kind can sit.

**Inheritance.** A child takes over the fields its kind lists as inherited
from the nearest item above it of its own kind, unless it sets them itself; a
value the child sets replaces the inherited one whole. Topics in between are
passed over and pass nothing on. `id`, `title`, `text`, `children`, `status`,
`mirrors`, `notes`, `links` and `retired` are never inherited, since they
describe the item itself.

**Mirrors.** Every item of every set this one depends on, topics aside,
applies here too, and has exactly one mirror in this set:

- An item without a mirror is a warning. Nothing below a requirement mirrored
  as waived needs a mirror of its own. Two mirrors of the same item are an
  error.
- Only requirements can be waived: the objectives, stakeholders and needs of a
  set this one depends on always apply.
- A mirror sits below the mirror of the nearest item above the one it
  mirrors, topics passed over, and topics of this set may stand between the
  two; a mirror elsewhere is a warning. The mirror of an item with only topics
  above it, or nothing, may sit anywhere.
- A mirror's `title` is the title of the item it mirrors, and a different
  title is a warning. Its `text` and its other fields say what the item means
  in this set's own context.

### Objective — prefix `O`

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

### Stakeholder class — prefix `S`

A role whose interest the product serves, such as a tester or an operator. A
class is a role, not a person.

| Field | Meaning |
|---|---|
| `text` | The class's interest in the product. |

Inherited by children: nothing.

```yaml
# good: a role, and what it wants from the product
- id: S3
  title: Office User
  text: Wants coffee without having to look after the machine.

# bad: a person, whose interest goes when they leave
- id: S4
  title: John From Facilities
  text: Asked for the machine.
```

### Need — prefix `N`

Something stakeholder classes need from the product, in the terms of a class
that has it. Needs are where requirements come from.

A need's classes are the nearest class above it, through needs and topics,
and those its `stakeholders` field names. A need is usually held by the one class that has
it; a need that several classes share can sit under one of them and name the
others, or sit under none and name them all.

| Field | Meaning |
|---|---|
| `text` | The need, as the class would say it. |
| `stakeholders` | Optional. The classes of this set that have it, besides the nearest one above it. A need with no class either way is a warning, so that a need can be written before its classes are placed; naming the nearest class above it is a warning too, since it says it twice. |
| `serves` | The objectives of this set it advances. The list may also hold the word `mission`, for a need that serves the mission directly. |
| `status` | `hypothesis` until real use confirms it, then `validated`. A need found to be wrong is retired. |

Inherited by children: `stakeholders`, `serves`.

```yaml
# good: the user's words, with no solution in them
- id: S3
  title: Office User
  text: Wants coffee without having to look after the machine.
  children:
    - id: N3.1
      title: Know Before Brewing
      text: Know before I brew that the tank needs water.
      serves: [O1]
      status: hypothesis

    # bad: a design in the system's words; the user never asked for a float
    - id: N3.2
      title: Float Sensor
      text: The tank has a float sensor.
      serves: [O1]
      status: hypothesis
```

### Requirement — prefix `R`

One promise the product keeps, specific enough to verify.

| Field | Meaning |
|---|---|
| `text` | The promise, as one sentence. |
| `serves` | The needs or requirements it exists for, in this set or another. Leaving it out is a warning, so that a requirement can be written before its reason is placed. |
| `rationale` | Optional. Why the promise exists, including options considered and not taken. Requirements that rest on the same reason serve the requirement that states it. |
| `verify` | Optional. How the promise is shown to hold: `test` (the default), `inspection`, `analysis` or `demonstration`. |
| `status` | Where the promise stands, required except under a waived requirement: `draft` is written but not yet agreed; `approved` is agreed as a promise the product will keep; `implemented` means the product does it; `verified` means it has been shown to hold by its `verify` method; `waived` means it does not apply to this product. |

Inherited by children: `serves`, `rationale`, `verify`.

A requirement's **effective status** is the lowest of its own `status` and the
effective status of the requirements nearest below it, through topics, in the
order `draft`, `approved`, `implemented`, `verified`. A requirement written as
`verified` with one child still `implemented` is effectively `implemented`;
both are shown, and the difference is not a fault. A `waived` requirement is
effectively waived together with every requirement below it, which need no
status of their own, and a waived requirement does not lower the one above it.

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
- id: R7
  title: Refill Lamp
  text: When the tank becomes empty, the machine SHALL light the refill lamp.
  serves: [N3.1]
  status: approved

# bad: "quickly" cannot be tested, and it makes two promises in one
- id: R8
  title: Water Handling
  text: The machine SHALL detect low water quickly and alert the user.
  serves: [N3.1]
  status: draft
```

### Topic — prefix `T`

What a reader needs to know to follow the items under it and after it: the
terms they use, the design they belong to, how its parts behave. A topic
explains and groups; it promises nothing and has no status.

| Field | Meaning |
|---|---|
| `text` | Optional. The explanation, in Markdown. Without it, the topic is a heading over its children; a topic with neither is a warning, since it explains and groups nothing. |

Inherited by children: nothing.

A topic's text is Markdown as CommonMark reads it, with GitHub's tables. HTML
in it is shown as text, and an image as a link to it. A code block marked
`mermaid` is a diagram, such as a state machine or a sequence of messages,
drawn as Mermaid draws it, and one marked `dot` a graph drawn as Graphviz
draws it. Nothing checks a diagram, so a mistake in one shows where it is
drawn. Every other field in a set is plain text.

````yaml
# good: the terms the requirements below use, before they use them
- id: T5
  title: Water Level
  text: |
    The tank is *empty* once the water is below the pump's intake, and *low*
    when less than one brew is left.

    ```mermaid
    stateDiagram-v2
      Full --> Low: brews
      Low --> Empty: a brew
      Empty --> Full: refill
    ```
  children:
    - id: R5.1
      title: Refill Lamp
      text: When the tank becomes empty, the machine SHALL light the refill lamp.
      serves: [N3.1]
      status: approved

# bad: a promise dressed as an explanation, which nothing traces or verifies
- id: T6
  title: Lid
  text: The lid must stay locked while a brew runs.
````

## References

| Field | On | Names |
|---|---|---|
| `ranking` | mission file | objectives of this set |
| `serves` | need | objectives of this set, and the word `mission` |
| `stakeholders` | need | stakeholder classes of this set |
| `serves` | requirement | needs or requirements, of this set or another |
| `mirrors` | any item but a topic | one item of the same kind in a set this one depends on |

No field names a topic. An item of another set is named `name/id`, where
`name` is the name the manifest gives that set under `depends`. A standard
that every product of a company or a department meets is a set of its own,
which each product depends on and mirrors.

**Citations.** A text cites an item by its id in square brackets: `[T2]`, or
`[company/R1]` for an item of a set this one depends on.

- Every prose field can cite: an item's `text`, `measure`, `rationale` and
  `notes`, and the mission statement. Code in a topic's Markdown does not.
- A citation can name an item of any kind, topics included, and counts as
  naming it.
- A citation that names no item, a retired one, an item under another
  letter, or a set the manifest does not depend on is a warning.
- A range is written as its two ends, each cited: `[R8.1.2]–[R8.1.4]`.

YAML reads a `[` that begins a value, or that stands anywhere in a value
written within braces, as the start of a list; such a value goes in quotes:
`text: "[T2] defines it."`.

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
title: Why The Machine Exists
mission: Coffee in the office without anyone looking after the machine.
ranking: [O1]
items:
  - id: O1
    title: No Dry Brews
    text: No brew starts while the water tank is empty.
    measure: brews started on an empty tank, per thousand brews.
  - id: S2
    title: Office User
    text: Wants coffee without having to look after the machine.
    children:
      - id: N2.1
        title: Know Before Brewing
        text: Know before I brew that the tank needs water.
        serves: [O1]
        status: validated
        notes:
          - Heard from three users in the first week of the trial.
```

```yaml
# requirements/01-tank.shall.yaml
title: Water Tank
items:
  - id: T3
    title: Water Level
    text: |
      The tank is *empty* once the water is below the pump's intake.
    children:
      - id: R3.1
        title: Refill Lamp
        text: When the tank becomes empty, the machine SHALL light the refill lamp.
        serves: [N2.1]
        status: implemented
        links:
          - url: https://ci.example.com/runs/4711
            title: Tank tests on hardware revision C
        children:
          - id: R3.1.1
            title: Lamp Seen From The Door
            text: The machine SHALL show the refill lamp to anyone standing at the
              kitchen door.
            verify: inspection
            status: implemented
        retired:
          - id: R3.1.2
            title: Refill Beep
retired:
  - id: R4
    title: Water Handling
```

## Schema

[`schema/format-1/`](../schema/format-1/) gives the shape of each file as JSON
Schema, for editors and other tools. It cannot express every rule here, such
as a child's id extending its parent's or a reference resolving; where the two
differ, this document holds.
