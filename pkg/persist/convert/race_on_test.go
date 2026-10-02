//go:build race

package convert_test

// raceDetector is whether the tests run under the race detector, which
// is about eight times slower: TestFixturesRoundTrip then takes every
// fifth fixture, and the plain and coverage runs take them all.
const raceDetector = true
