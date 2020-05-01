# State Machine Best Practices

This repo acts as a best-practices guide for implementing state machines in
golang using an object-oriented approach. Take or leave these design decisions,
add to them or subtract, just believe in your design.

## Principles

- Each state is its own object
- Each state inherits functionality from a common base state
- The state runner is as simple as possible using interface functions only
- States can inherently retry themselves
- States can conditionally choose which states come next
- The state runner can run the state machine infinitely or stop when a state
  identifies as the last state
- A state can run concurrent actions as part of the base operation

## Usage

The interface in this repo may be used by state machine runners if desired, but
its usage is not required. This repo will follow www.semver.org principles so
any breaking changes will result in a major version bump and a new import path.
