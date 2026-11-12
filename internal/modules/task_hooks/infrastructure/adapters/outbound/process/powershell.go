package process

import "strings"

// powershellPositionals are the spellings with which a PowerShell command line reads its own arguments. @args is
// listed too: a line that already splats them must not get them a second time.
var powershellPositionals = []string{"$args", "@args", "$input"}

// powershellQuotes are the characters PowerShell reads as a single quote: the ASCII one and the typographic ones.
// A quote of any of them inside a literal would close it, so each is doubled.
const powershellQuotes string = "'‘’‚‛"

// quotePowerShell returns arg as a single-quoted literal that PowerShell never parses.
func quotePowerShell(arg string) string {
	var quoted strings.Builder
	quoted.WriteByte('\'')
	for _, r := range arg {
		quoted.WriteRune(r)
		if strings.ContainsRune(powershellQuotes, r) {
			quoted.WriteRune(r)
		}
	}
	quoted.WriteByte('\'')
	return quoted.String()
}

// powershellScript wraps the command line in a script block called with the arguments as quoted literals, so the
// line reads them as $args. PowerShell reduces the outcome of -Command to 0 or 1, so the script ends with the exit
// code of the failed command (or 1 when it has none, as with a cmdlet), keeping the real exit code of a program.
func powershellScript(command string, args []string) string {
	var script strings.Builder
	script.WriteString("& {\n")
	script.WriteString(command)
	script.WriteString("\n}")
	for _, arg := range args {
		script.WriteString(" " + quotePowerShell(arg))
	}
	script.WriteString("\n$ok = $?\nif ($ok) { exit 0 }\nif ($LASTEXITCODE) { exit $LASTEXITCODE }\nexit 1\n")
	return script.String()
}
