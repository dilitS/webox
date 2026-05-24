// Package components hosts reusable presentation primitives shared by
// the Bento engine, the Standard cockpit, and the wizard screens.
//
// The package is intentionally tiny: each component is a pure function
// (or a tiny stateless struct) that takes a [theme.Theme] plus its data
// and returns a rendered string. Bubble Tea state machines compose these
// rather than re-implementing borders, gradients, and badges in every
// view file.
//
// Sprint 08 ships three primitives:
//
//   - [HeaderBar]: gradient title + mode badge used at the top of the
//     cockpit.
//   - [Spinner]: adaptive spinner that switches frame style based on
//     viewport ("dot" for Standard, "pulse" for Ultra) so larger
//     terminals get a more cinematic indicator.
//   - [Modal]: double-border centred dialog used for confirmation
//     prompts and rollback summaries.
package components
