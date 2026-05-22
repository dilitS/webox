// Package wizard runs multi-step orchestrations such as project
// creation, with rollback-on-failure semantics.
//
// MVP uses a LIFO rollback stack persisted to pending_cleanups.json
// so an interrupted wizard can resume cleanly on the next launch.
// Each step pushes a Compensator that knows how to undo its effect;
// a failure pops the stack and runs each Compensator in reverse order.
// The DAG-based transactional engine described in docs/DESIGN.md §10
// is target architecture for v0.3+.
package wizard
