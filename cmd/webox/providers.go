package main

// This file holds the blank imports that wire production hosting
// provider adapters into the binary so their init() blocks register
// the factories with `providers.Register` before [Run] looks them up.
//
// `webox provider new <name>` re-sorts the import block below on
// every invocation. Hand-editing is supported — merge conflicts on
// this file resolve trivially because the block is alphabetically
// sorted.

import (
	_ "github.com/dilitS/webox/providers/smallhost" // register smallhost factory
)
