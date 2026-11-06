package orders

// Only the template catalog knows the codes of the industry templates (INV-9).
func defaultTemplate(carpentry bool) string {
	if carpentry {
		return "aluminum_carpentry"
	}
	return "generic"
}

const rawCode = `generic`

const splitCode = "gener" + "ic"

const multilineCode = "" +
	"generic"

const multilineSplitCode = "gener" +
	"ic" +
	""
