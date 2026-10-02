# Switchyard design system — PX3

## Identity

Switchyard is dark-first and uses its actual rail-switch mark. The authenticated product and brochure share a visual family without pretending to be the same kind of page.

## Palette

Main surfaces are near-black charcoal/graphite rather than pure black:

```text
background       #0b0c0f
panel            #14161b
elevated         #191c22
input             #0f1115
code              #08090c
border            #2a2e36
text              #ece9e2
muted             #999da7
```

Functional/identity accents:

```text
rail amber        #d89a43
strong amber      #efad52
signal green      #55b58b
warning amber     #d8a04e
muted red         #c9695d
```

Blue is not a Switchyard brand or primary interaction color. Later syntax-highlighting work may use a broad token palette where technically useful, but product controls/navigation/status do not depend on GitHub blue.

## Components

- restrained 1px borders rather than floating glass cards;
- minimal shadow/elevation;
- real mark + wordmark treatment in headers;
- amber for primary actions/focus, green for successful status, red only for danger;
- code surfaces one step darker than the application shell;
- consistent focus-visible treatment;
- 6/9/12px radius scale;
- dense developer-tool layouts remain readable rather than oversized SaaS dashboards.

## Accessibility

Color never carries state alone. Focus rings are visible against all main backgrounds. Reduced motion disables nonessential transitions. Text/background contrast remains high on the primary surfaces.
