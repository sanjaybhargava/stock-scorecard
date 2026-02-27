package matcher

// KnownISINRenames maps old ISINs to new ISINs for cases where a corporate
// action (e.g. merger) assigned a completely new ISIN. Most "renames" like
// MOTHERSUMI→MOTHERSON keep the same ISIN and are already handled by
// ISIN-based FIFO matching. This map covers the rare edge case.
//
// Populate from real data as users encounter mismatches.
var KnownISINRenames = map[string]string{
	// Example: "INE001A01036": "INE040A01034", // HDFC→HDFCBANK merger
}
