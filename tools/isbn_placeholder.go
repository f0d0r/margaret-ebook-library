package tools

// PlaceholderKind classifies why an ISBN is considered a placeholder.
type PlaceholderKind uint8

const (
	NotPlaceholder PlaceholderKind = iota
	PlaceholderAllSameDigits
	PlaceholderSequentialDigits
	PlaceholderRepeatedPattern
	PlaceholderLowEntropy
	PlaceholderKnownTestValue
)

// placeholderTestCores lists nine-digit cores that occur in test data and
// examples rather than in issued book identifiers.
var placeholderTestCores = map[string]struct{}{
	"123456789": {},
	"987654321": {},
	"012345678": {},
	"214748364": {},
	"429496729": {},
	"314159265": {},
	"271828182": {},
	"161803398": {},
	"123123123": {},
	"111222333": {},
	"101010101": {},
}

// PlaceholderKind reports why i is considered a placeholder. A zero or
// malformed ISBN returns NotPlaceholder.
func (i ISBN) PlaceholderKind() PlaceholderKind {
	core, ok := i.placeholderCore()
	if !ok {
		return NotPlaceholder
	}
	switch {
	case allSameDigits(core):
		return PlaceholderAllSameDigits
	case isKnownTestCore(core):
		return PlaceholderKnownTestValue
	case sequentialDigits(core):
		return PlaceholderSequentialDigits
	case repeatedPattern(core) || hasDigitRun(core, 6):
		return PlaceholderRepeatedPattern
	case distinctDigits(core) <= 2:
		return PlaceholderLowEntropy
	default:
		return NotPlaceholder
	}
}

func (i ISBN) placeholderCore() (string, bool) {
	switch i.Kind() {
	case ISBN10:
		return i.digits[:9], true
	case ISBN13:
		return i.digits[3:12], true
	default:
		return "", false
	}
}

func isKnownTestCore(core string) bool {
	_, ok := placeholderTestCores[core]
	return ok
}

func allSameDigits(core string) bool {
	if core == "" {
		return false
	}
	for i := 1; i < len(core); i++ {
		if core[i] != core[0] {
			return false
		}
	}
	return true
}

func sequentialDigits(core string) bool {
	if len(core) < 2 {
		return false
	}
	up, down := true, true
	for i := 1; i < len(core); i++ {
		prev := int(core[i-1] - '0')
		cur := int(core[i] - '0')
		if cur != (prev+1)%10 {
			up = false
		}
		if cur != (prev+9)%10 {
			down = false
		}
	}
	return up || down
}

func repeatedPattern(core string) bool {
	for _, period := range []int{2, 3} {
		ok := true
		for i := period; i < len(core); i++ {
			if core[i] != core[i-period] {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	if len(core) != 9 {
		return false
	}
	for start := 0; start < len(core); start += 3 {
		if core[start] != core[start+1] || core[start] != core[start+2] {
			return false
		}
	}
	return true
}

func hasDigitRun(core string, length int) bool {
	run := 1
	for i := 1; i < len(core); i++ {
		if core[i] == core[i-1] {
			run++
			if run >= length {
				return true
			}
			continue
		}
		run = 1
	}
	return false
}

func distinctDigits(core string) int {
	var seen [10]bool
	count := 0
	for i := 0; i < len(core); i++ {
		d := core[i] - '0'
		if !seen[d] {
			seen[d] = true
			count++
		}
	}
	return count
}
