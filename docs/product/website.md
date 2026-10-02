# Website: application vs brochure boundary

Two distinct properties exist and must not be conflated.

## `switchyard-labs/switchyard` — the product

The actual Switchyard product, including the **authenticated GitHub-like
application UI** documented in `application.md`. This repository holds the
product documentation and (later) the product implementation.

## `switchyard-labs/switchyard-labs.github.io` — the brochure site

A promotional/informational website **about** Switchyard:

- what Switchyard is;
- why it exists;
- architecture/philosophy;
- screenshots/demos;
- getting started;
- public docs;
- comparisons;
- project information.

## Rules

- The two can share design language and branding, but they have **different
  jobs** and are different builds/deployments.
- The brochure site must not grow into the authenticated application.
- The application must not depend on the brochure site for product functionality.
- The brochure is informational/marketing; the application is the operational
  product.
- The website (both) is built with Nift as appropriate; Nift is consumed, not
  modified ad hoc.