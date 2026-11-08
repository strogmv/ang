package locationsdk

// PublicServiceApproach is the shared public queue/entry datum used by the
// service compiler and source-art locators. Approach order is authored: the
// last cell is the public waiting point, farther from the bound doorway.
func PublicServiceApproach(port CompiledPort) (Cell, bool) {
	if len(port.Approach) == 0 {
		return Cell{}, false
	}
	return port.Approach[len(port.Approach)-1], true
}
