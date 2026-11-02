package dbbootstrap

import (
	"fmt"
	"regexp"
	"strings"
)

var migrationACLString = regexp.MustCompile(`(?is)EXECUTE\s+'((?:''|[^'])*)'`)
var migrationACLLiteral = regexp.MustCompile(`(?is)'(?:''|[^'])*'`)
var migrationACLWord = regexp.MustCompile(`(?is)\b(?:GRANT|REVOKE)\b`)
var migrationACLFormat = regexp.MustCompile(`(?is)EXECUTE\s+format\s*\([^;]*\b(?:GRANT|REVOKE)\b`)
var migrationACLFormatCall = regexp.MustCompile(`(?is)\bEXECUTE\s+format\s*\([^;]*\)`)
var migrationACLFormatRole = regexp.MustCompile(`(?is)EXECUTE\s+format\s*\([^;]*\b(?:GRANT|REVOKE)\b[^;]*\b(?:TO|FROM)\s*%`)
var migrationACLExplicitTarget = regexp.MustCompile(`(?is)\b(?:TO|FROM)\s+(?:PUBLIC|[a-z_][a-z0-9_]*)\b`)
var migrationACLFormatTarget = regexp.MustCompile(`(?is)EXECUTE\s+format\s*\([^;]*\b(?:GRANT|REVOKE)\b[^;]*\b(?:TO|FROM)\s+PUBLIC\b`)
var migrationACLExecWord = regexp.MustCompile(`(?i)\bEXECUTE\b`)
var migrationACLTriggerExec = regexp.MustCompile(`(?is)\bEXECUTE\s+FUNCTION\b`)
var migrationACLStaticExecPrivilege = regexp.MustCompile(`(?is)\b(?:GRANT|REVOKE)\s+EXECUTE\b`)
var migrationACLExecConcat = regexp.MustCompile(`(?is)\bEXECUTE\s+'(?:''|[^'])*'\s*(?:\|\||\+)`)
var migrationACLExecACLConcat = regexp.MustCompile(`(?is)\bEXECUTE\s+'(?:''|[^'])*\b(?:GRANT|REVOKE)\b(?:''|[^'])*'\s*(?:\|\||\+)`)
var migrationACLExecClause = regexp.MustCompile(`(?is)\bEXECUTE\s+'(?:''|[^'])*'\s+(?:INTO|USING)\b`)

func migrationACLRefForGrantee(grantee string) *regexp.Regexp {
	return regexp.MustCompile(`(?is)\b(?:GRANT|REVOKE)\b[^;]*?\b(?:TO|FROM)\s+` + regexp.QuoteMeta(grantee) + `\b[^;]*`)
}

func migrationACLExecutableSources(source, grantee string) ([]string, []string, error) {
	statements, err := splitACLStatements(source)
	if err != nil {
		return nil, nil, err
	}
	targetRef := migrationACLRefForGrantee(grantee)
	var executable []string
	var proceduralBodies []string
	for _, statement := range statements {
		masked := maskACLQuotedTokens(statement)
		if loc := migrationACLWord.FindStringIndex(masked); loc != nil && strings.TrimSpace(masked[:loc[0]]) == "" {
			executable = append(executable, statement)
			continue
		}
		body, isDO, foundBody, err := migrationACLDOBody(statement)
		if err != nil {
			return nil, nil, err
		}
		if isDO && !foundBody && targetRef.MatchString(statement) {
			return nil, nil, fmt.Errorf("unsupported DO body containing ACL for %s", grantee)
		}
		if !foundBody {
			continue
		}
		bodyStatements, err := splitACLStatements(body)
		if err != nil {
			return nil, nil, err
		}
		proceduralBodies = append(proceduralBodies, strings.Join(bodyStatements, ";"))
		for _, bodyStatement := range bodyStatements {
			maskedBody := maskACLQuotedTokens(bodyStatement)
			for _, loc := range migrationACLWord.FindAllStringIndex(maskedBody, -1) {
				executable = append(executable, bodyStatement[loc[0]:])
			}
		}
	}
	return executable, proceduralBodies, nil
}

func migrationACLDOBody(statement string) (body string, isDO, found bool, err error) {
	trimmed := strings.TrimSpace(statement)
	isDO = regexp.MustCompile(`(?is)^DO\b`).MatchString(trimmed)
	if !isDO {
		return "", false, false, nil
	}
	for i := 0; i < len(trimmed); i++ {
		if trimmed[i] != '$' {
			continue
		}
		tag := aclDollarQuote.FindString(trimmed[i:])
		if tag == "" {
			continue
		}
		start := i + len(tag)
		end := strings.Index(trimmed[start:], tag)
		if end < 0 {
			return "", true, false, fmt.Errorf("unterminated DO body dollar quote")
		}
		return trimmed[start : start+end], true, true, nil
	}
	return "", true, false, nil
}

func maskACLQuotedTokens(source string) string {
	masked := []byte(source)
	for i := 0; i < len(source); {
		switch source[i] {
		case '\'':
			start := i
			escape := isACLEscapeStringStart(source, i)
			i++
			for i < len(source) {
				if escape && source[i] == '\\' && i+1 < len(source) {
					i += 2
					continue
				}
				if source[i] == '\'' {
					if i+1 < len(source) && source[i+1] == '\'' {
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
			for j := start; j < i; j++ {
				masked[j] = ' '
			}
		case '"':
			start := i
			i++
			for i < len(source) {
				if source[i] == '"' {
					if i+1 < len(source) && source[i+1] == '"' {
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
			for j := start; j < i; j++ {
				masked[j] = ' '
			}
		case '$':
			tag := aclDollarQuote.FindString(source[i:])
			if tag == "" {
				i++
				continue
			}
			start := i
			bodyStart := i + len(tag)
			end := strings.Index(source[bodyStart:], tag)
			if end < 0 {
				i++
				continue
			}
			i = bodyStart + end + len(tag)
			for j := start; j < i; j++ {
				masked[j] = ' '
			}
		default:
			i++
		}
	}
	return string(masked)
}

// ExtractMigrationRuntimeACL extracts the explicitly supported runtime-role ACL
// forms from immutable migration source. It does not execute SQL or inspect a DB.
func ExtractMigrationRuntimeACL(version int, source []byte) ([]RuntimeACLEffect, error) {
	return extractMigrationACL(version, source, "espacigo_runtime", false)
}

// ExtractMigrationACLForGrantee extracts supported ACL statements for one role
// or PUBLIC. Matching is based on the GRANT/REVOKE target clause, not object
// names or schemas that happen to contain the same word.
func ExtractMigrationACLForGrantee(version int, source []byte, grantee string) ([]RuntimeACLEffect, error) {
	grantee, err := normalizeACLGrantee(grantee)
	if err != nil {
		return nil, err
	}
	return extractMigrationACL(version, source, grantee, true)
}

func extractMigrationACL(version int, source []byte, grantee string, includeGrantee bool) ([]RuntimeACLEffect, error) {
	if _, err := splitACLStatements(string(source)); err != nil {
		return nil, fmt.Errorf("migration V%d has unsupported SQL source: %w", version, err)
	}
	executableSources, proceduralBodies, err := migrationACLExecutableSources(string(source), grantee)
	if err != nil {
		return nil, fmt.Errorf("migration V%d has unsupported ACL source: %w", version, err)
	}
	dynamicSource := strings.Join(proceduralBodies, ";")
	if err := rejectUnsupportedMigrationACL(version, dynamicSource, grantee); err != nil {
		return nil, err
	}
	var out []RuntimeACLEffect
	targetRef := migrationACLRefForGrantee(grantee)
	scan := func(s string) error {
		for _, loc := range targetRef.FindAllStringIndex(s, -1) {
			stmt := strings.TrimSpace(s[loc[0]:loc[1]])
			// Tolerate a simple IF EXISTS wrapper by isolating the SQL ACL statement.
			if i := migrationACLWord.FindStringIndex(stmt); i != nil {
				stmt = stmt[i[0]:]
			}
			var effects []RuntimeACLEffect
			var err error
			if includeGrantee {
				effects, err = ParseACLStatementsForGrantee(stmt, grantee)
			} else {
				effects, err = ParseRuntimeACLStatements(stmt)
			}
			if err != nil {
				return fmt.Errorf("migration V%d unsupported ACL for %s near %q: %w", version, grantee, stmt, err)
			}
			out = append(out, effects...)
		}
		return nil
	}
	// Scan only executable ACL statements and procedural code bodies. Quoted
	// data literals must not be mistaken for migration commands.
	for _, executableSource := range executableSources {
		if err := scan(executableSource); err != nil {
			return nil, err
		}
	}
	for _, body := range proceduralBodies {
		for _, m := range migrationACLString.FindAllStringSubmatch(body, -1) {
			decoded := strings.ReplaceAll(m[1], "''", "'")
			if targetRef.MatchString(decoded) {
				if err := scan(decoded); err != nil {
					return nil, err
				}
			}
		}
	}
	// Dynamic format SQL cannot be interpreted safely when it targets this
	// grantee or interpolates a grantee after TO/FROM.
	if grantee == "espacigo_runtime" && migrationACLFormat.MatchString(dynamicSource) && strings.Contains(strings.ToLower(dynamicSource), grantee) ||
		migrationACLFormatTarget.MatchString(dynamicSource) || migrationACLFormatRole.MatchString(dynamicSource) {
		return nil, fmt.Errorf("migration V%d has unsupported dynamic ACL syntax for %s", version, grantee)
	}
	return out, nil
}

func rejectUnsupportedMigrationACL(version int, text, grantee string) error {
	lower := strings.ToLower(text)
	targetRef := migrationACLRefForGrantee(grantee)
	if grantee != "espacigo_runtime" && migrationACLExecACLConcat.MatchString(text) {
		return fmt.Errorf("migration V%d has unsupported dynamic ACL syntax for %s", version, grantee)
	}
	if grantee == "espacigo_runtime" && !strings.Contains(lower, grantee) || grantee != "espacigo_runtime" && !targetRef.MatchString(text) && !migrationACLFormatRole.MatchString(text) {
		return nil
	}
	if migrationACLExecConcat.MatchString(text) || migrationACLExecClause.MatchString(text) {
		return fmt.Errorf("migration V%d has unsupported dynamic ACL syntax for %s", version, grantee)
	}
	for _, match := range migrationACLString.FindAllStringSubmatch(text, -1) {
		decoded := strings.ReplaceAll(match[1], "''", "'")
		if migrationACLWord.MatchString(decoded) && !targetRef.MatchString(decoded) &&
			(grantee == "espacigo_runtime" || !migrationACLExplicitTarget.MatchString(decoded)) {
			return fmt.Errorf("migration V%d has unsupported dynamic ACL syntax for %s", version, grantee)
		}
	}
	dynamic := migrationACLString.ReplaceAllString(text, " ")
	dynamic = migrationACLLiteral.ReplaceAllString(dynamic, " ")
	dynamic = migrationACLTriggerExec.ReplaceAllString(dynamic, " ")
	dynamic = migrationACLStaticExecPrivilege.ReplaceAllString(dynamic, " ")
	dynamic = migrationACLFormatCall.ReplaceAllString(dynamic, " ")
	if grantee != "espacigo_runtime" && targetRef.MatchString(text) && migrationACLExecWord.MatchString(dynamic) {
		return fmt.Errorf("migration V%d has unsupported dynamic ACL syntax for %s", version, grantee)
	}
	if grantee == "espacigo_runtime" && (migrationACLExecWord.MatchString(dynamic) || migrationACLFormat.MatchString(text)) {
		return fmt.Errorf("migration V%d has unsupported dynamic ACL syntax for %s", version, grantee)
	}
	return nil
}
