package dbbootstrap

import (
	"fmt"
	"regexp"
	"strings"
)

// RuntimeACLEffect is one normalized object/privilege ACL action parsed from the grant source.
type RuntimeACLEffect struct {
	Action, ObjectKind, Object, Privilege, Column string
	Grantee                                       string
	GrantOption                                   bool
	RevokeBehavior                                string
	Grantor                                       string
	Condition                                     string
	DynamicSQL                                    bool
}

var aclColumn = regexp.MustCompile(`(?is)^([a-z ]+)\s*\(([^()]*)\)$`)
var aclIdent = regexp.MustCompile(`^[a-z_][a-z0-9_]*(\.[a-z_][a-z0-9_]*)?$`)
var aclRoleIdent = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)
var aclFunctionTypes = map[string]struct{}{
	"bigint":      {},
	"integer":     {},
	"timestamptz": {},
	"uuid":        {},
}

var aclDollarQuote = regexp.MustCompile(`^\$(?:[a-zA-Z_][a-zA-Z0-9_]*)?\$`)

func isACLEscapeStringStart(source string, quoteIndex int) bool {
	if quoteIndex == 0 || (source[quoteIndex-1] != 'E' && source[quoteIndex-1] != 'e') {
		return false
	}
	if quoteIndex == 1 {
		return true
	}
	previous := source[quoteIndex-2]
	return !((previous >= 'a' && previous <= 'z') ||
		(previous >= 'A' && previous <= 'Z') ||
		(previous >= '0' && previous <= '9') || previous == '_' || previous == '$')
}

func splitACLStatements(source string) ([]string, error) {
	var statements []string
	var current strings.Builder
	inSingle, inDouble, inLineComment, inEscapeString := false, false, false, false
	blockDepth := 0
	dollarTag := ""
	for i := 0; i < len(source); {
		if inLineComment {
			if source[i] == '\n' {
				current.WriteByte(' ')
				inLineComment = false
			}
			i++
			continue
		}
		if blockDepth > 0 {
			switch {
			case strings.HasPrefix(source[i:], "/*"):
				blockDepth++
				i += 2
			case strings.HasPrefix(source[i:], "*/"):
				blockDepth--
				i += 2
			default:
				i++
			}
			continue
		}
		if dollarTag != "" {
			if strings.HasPrefix(source[i:], dollarTag) {
				current.WriteString(dollarTag)
				i += len(dollarTag)
				dollarTag = ""
				continue
			}
			current.WriteByte(source[i])
			i++
			continue
		}
		if inSingle {
			current.WriteByte(source[i])
			if inEscapeString && source[i] == '\\' && i+1 < len(source) {
				current.WriteByte(source[i+1])
				i += 2
				continue
			}
			if source[i] == '\'' {
				if i+1 < len(source) && source[i+1] == '\'' {
					current.WriteByte(source[i+1])
					i += 2
					continue
				}
				inSingle = false
				inEscapeString = false
			}
			i++
			continue
		}
		if inDouble {
			current.WriteByte(source[i])
			if source[i] == '"' {
				if i+1 < len(source) && source[i+1] == '"' {
					current.WriteByte(source[i+1])
					i += 2
					continue
				}
				inDouble = false
			}
			i++
			continue
		}
		switch {
		case strings.HasPrefix(source[i:], "--"):
			current.WriteByte(' ')
			inLineComment = true
			i += 2
		case strings.HasPrefix(source[i:], "/*"):
			current.WriteByte(' ')
			blockDepth = 1
			i += 2
		case source[i] == '\'':
			inSingle = true
			inEscapeString = isACLEscapeStringStart(source, i)
			current.WriteByte(source[i])
			i++
		case source[i] == '"':
			inDouble = true
			current.WriteByte(source[i])
			i++
		case source[i] == '$':
			tag := aclDollarQuote.FindString(source[i:])
			if tag == "" {
				current.WriteByte(source[i])
				i++
				continue
			}
			current.WriteString(tag)
			dollarTag = tag
			i += len(tag)
		case source[i] == ';':
			statements = append(statements, current.String())
			current.Reset()
			i++
		default:
			current.WriteByte(source[i])
			i++
		}
	}
	if inSingle || inDouble || blockDepth > 0 || dollarTag != "" {
		return nil, fmt.Errorf("unterminated SQL quote or comment")
	}
	if tail := strings.TrimSpace(current.String()); tail != "" {
		statements = append(statements, current.String())
	}
	return statements, nil
}

// ParseRuntimeACLStatements parses only the deliberately supported runtime GRANT/REVOKE grammar.
func ParseRuntimeACLStatements(source string) ([]RuntimeACLEffect, error) {
	effects, err := parseACLStatementsForGrantee(source, "espacigo_runtime")
	if err != nil {
		return nil, err
	}
	for i := range effects {
		effects[i].Grantee = ""
	}
	return effects, nil
}

// ParseACLStatementsForGrantee parses the supported GRANT/REVOKE grammar for
// one unquoted lowercase role identifier or PUBLIC. It is intentionally not a
// general PostgreSQL parser and rejects quoted role names.
func ParseACLStatementsForGrantee(source, grantee string) ([]RuntimeACLEffect, error) {
	var err error
	grantee, err = normalizeACLGrantee(grantee)
	if err != nil {
		return nil, err
	}
	effects, err := parseACLStatementsForGrantee(source, grantee)
	if err != nil {
		return nil, err
	}
	for i := range effects {
		effects[i].Grantee = grantee
	}
	return effects, nil
}

func normalizeACLGrantee(grantee string) (string, error) {
	grantee = strings.TrimSpace(grantee)
	if strings.EqualFold(grantee, "PUBLIC") {
		return "PUBLIC", nil
	}
	if !aclRoleIdent.MatchString(grantee) {
		return "", fmt.Errorf("invalid ACL grantee")
	}
	return grantee, nil
}

func parseACLStatementsForGrantee(source, grantee string) ([]RuntimeACLEffect, error) {
	var out []RuntimeACLEffect
	statementPattern := regexp.MustCompile(`(?is)^(GRANT|REVOKE)\s+(.+?)\s+(ON\s+(TABLE|SEQUENCE|FUNCTION|SCHEMA)\s+|ON\s+)(.+?)\s+(TO|FROM)\s+` + regexp.QuoteMeta(grantee) + `(?:\s+(WITH GRANT OPTION))?(?:\s+(CASCADE|RESTRICT))?$`)
	statements, err := splitACLStatements(source)
	if err != nil {
		return nil, err
	}
	for _, raw := range statements {
		stmt := strings.TrimSpace(raw)
		if stmt == "" {
			continue
		}
		m := statementPattern.FindStringSubmatch(stmt)
		if m == nil {
			return nil, fmt.Errorf("unsupported ACL statement for %s: %q", grantee, stmt)
		}
		act, rawPrivileges, kind, objects, direction, option := strings.ToUpper(m[1]), strings.TrimSpace(m[2]), strings.ToUpper(m[4]), strings.TrimSpace(m[5]), strings.ToUpper(m[6]), strings.EqualFold(m[7], "WITH GRANT OPTION")
		revokeBehavior := strings.ToUpper(m[8])
		if act == "GRANT" && direction != "TO" || act == "REVOKE" && direction != "FROM" {
			return nil, fmt.Errorf("invalid ACL direction")
		}
		if act == "GRANT" && m[8] != "" || act == "REVOKE" && strings.EqualFold(m[7], "WITH GRANT OPTION") {
			return nil, fmt.Errorf("invalid action-specific ACL option")
		}
		if kind == "" {
			kind = "TABLE"
		}
		privilegeItems, err := splitACLList(rawPrivileges)
		if err != nil {
			return nil, fmt.Errorf("invalid privilege list: %w", err)
		}
		type privilegeScope struct {
			privilege string
			columns   []string
		}
		var scopes []privilegeScope
		for _, item := range privilegeItems {
			privilege := strings.ToUpper(strings.TrimSpace(item))
			var columns []string
			if cm := aclColumn.FindStringSubmatch(item); cm != nil {
				privilege = strings.ToUpper(strings.TrimSpace(cm[1]))
				columnNames, err := splitACLList(cm[2])
				if err != nil {
					return nil, fmt.Errorf("invalid column list: %w", err)
				}
				for _, column := range columnNames {
					columns = append(columns, strings.ToLower(strings.TrimSpace(column)))
				}
			}
			if privilege == "ALL" {
				privilege = "ALL PRIVILEGES"
			}
			scopes = append(scopes, privilegeScope{privilege: privilege, columns: columns})
		}
		objs, err := splitACLList(objects)
		if err != nil {
			return nil, fmt.Errorf("invalid object list: %w", err)
		}
		for _, o := range objs {
			o = strings.ToLower(strings.TrimSpace(o))
			if kind == "SCHEMA" {
				if !regexp.MustCompile(`^[a-z_][a-z0-9_]*$`).MatchString(o) {
					return nil, fmt.Errorf("invalid schema identity %q", o)
				}
			} else if kind == "FUNCTION" {
				if err := validateACLFunctionIdentity(o); err != nil {
					return nil, fmt.Errorf("invalid function identity %q", o)
				}
			} else if !aclIdent.MatchString(o) {
				return nil, fmt.Errorf("invalid object identity %q", o)
			}
			for _, scope := range scopes {
				priv := scope.privilege
				if !validACLPrivilege(kind, priv) {
					return nil, fmt.Errorf("unsupported privilege %q", priv)
				}
				if len(scope.columns) == 0 {
					out = append(out, RuntimeACLEffect{Action: act, ObjectKind: kind, Object: o, Privilege: priv, GrantOption: option, RevokeBehavior: revokeBehavior})
				} else {
					if kind != "TABLE" || priv == "ALL PRIVILEGES" || (priv != "SELECT" && priv != "INSERT" && priv != "UPDATE" && priv != "REFERENCES") {
						return nil, fmt.Errorf("column scope is invalid for %s %s", kind, priv)
					}
					for _, c := range scope.columns {
						if !regexp.MustCompile(`^[a-z_][a-z0-9_]*$`).MatchString(c) {
							return nil, fmt.Errorf("invalid column %q", c)
						}
						out = append(out, RuntimeACLEffect{Action: act, ObjectKind: kind, Object: o, Privilege: priv, Column: c, GrantOption: option, RevokeBehavior: revokeBehavior})
					}
				}
			}
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no ACL statements")
	}
	return out, nil
}

func validACLPrivilege(kind, privilege string) bool {
	switch kind {
	case "TABLE":
		switch privilege {
		case "SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER", "ALL PRIVILEGES":
			return true
		}
	case "SEQUENCE":
		switch privilege {
		case "USAGE", "SELECT", "UPDATE", "ALL PRIVILEGES":
			return true
		}
	case "FUNCTION":
		return privilege == "EXECUTE" || privilege == "ALL PRIVILEGES"
	case "SCHEMA":
		return privilege == "USAGE" || privilege == "CREATE" || privilege == "ALL PRIVILEGES"
	}
	return false
}

func splitACLList(value string) ([]string, error) {
	var items []string
	depth, start := 0, 0
	for i, r := range value {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return nil, fmt.Errorf("unbalanced parentheses")
			}
		case ',':
			if depth == 0 {
				items = append(items, strings.TrimSpace(value[start:i]))
				start = i + 1
			}
		}
	}
	if depth != 0 {
		return nil, fmt.Errorf("unbalanced parentheses")
	}
	items = append(items, strings.TrimSpace(value[start:]))
	for _, item := range items {
		if item == "" {
			return nil, fmt.Errorf("empty list item")
		}
	}
	return items, nil
}

func validateACLFunctionIdentity(identity string) error {
	open := strings.IndexByte(identity, '(')
	if open <= 0 || !strings.HasSuffix(identity, ")") || !aclIdent.MatchString(identity[:open]) {
		return fmt.Errorf("invalid function name")
	}
	args := strings.TrimSpace(identity[open+1 : len(identity)-1])
	if args == "" {
		return nil
	}
	argTypes, err := splitACLList(args)
	if err != nil {
		return err
	}
	for _, arg := range argTypes {
		if _, ok := aclFunctionTypes[strings.ToLower(strings.TrimSpace(arg))]; !ok {
			return fmt.Errorf("invalid argument type %q", arg)
		}
	}
	return nil
}
