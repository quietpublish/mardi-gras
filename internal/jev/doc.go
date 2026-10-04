// Package jev is a minimal client for TypeSafe AI's System One ("Jev") API
// and API-compatible servers such as OpenJev, Von or tensai.
//
// Jev answers typed questions about a JSON state instead of generating text:
// a yes/no probability (noul), one of a set of options (choice), or a point
// on an ordered scale (score), each with a calibrated confidence. mg uses it
// as a fast judge in places that are otherwise hand-written heuristics, and
// every consumer must work identically when Jev is off, slow or wrong.
//
// The package is stdlib-only and knows nothing about issues or agents:
// callers pass any JSON-marshallable state. It is imported by internal/app
// alone; other packages build the state and receive verdicts as plain values.
//
// The wire format follows the one mattn/go-jev v0.0.3 speaks. That SDK is
// not a dependency because its module requires a newer Go toolchain than this
// repository pins, and the whole protocol is a few struct tags.
//
// Jev is enabled only when MG_JEV_API_KEY is set (see FromEnv): issue text
// leaves the machine and each call costs money, so nothing is sent without
// the operator asking for it.
package jev
