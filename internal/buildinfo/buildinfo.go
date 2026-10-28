// Package buildinfo names the program variant selected at build time.
package buildinfo

// ProductName is the user-visible name of this build. The dev tag changes only
// the identity; data directories and vault keys stay shared with release builds.
const ProductName = productName

// Development reports whether this binary was built with the dev tag.
const Development = development
