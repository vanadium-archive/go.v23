package compile

import (
	"fmt"
	"regexp"
	"strconv"

	"v.io/core/veyron2/i18n"
	"v.io/core/veyron2/vdl/parse"
	"v.io/core/veyron2/verror"
	"v.io/core/veyron2/verror2"
)

// ErrorDef represents a user-defined error definition in the compiled results.
type ErrorDef struct {
	NamePos                    // name, parse position and docs
	ID      verror.ID          // error ID
	Action  verror2.ActionCode // action to be performed by client
	Params  []*Arg             // list of positional parameter names and types
	Formats []LangFmt          // list of language / format pairs
	English string             // English format text from Formats
}

// LangFmt represents a language / format string pair.
type LangFmt struct {
	Lang i18n.LangID // IETF language tag
	Fmt  string      // i18n format string in the given language.
}

func (x *ErrorDef) String() string {
	return fmt.Sprintf("%+v", *x)
}

// compileErrorDefs fills in pkg with compiled error definitions.
func compileErrorDefs(pkg *Package, pfiles []*parse.File, env *Env) {
	for index := range pkg.Files {
		file, pfile := pkg.Files[index], pfiles[index]
		for _, ped := range pfile.ErrorDefs {
			name, detail := ped.Name, identDetail("error", file, ped.Pos)
			if err := file.DeclareIdent(name, detail); err != nil {
				env.prefixErrorf(file, ped.Pos, err, "error %s name conflict", name)
				continue
			}
			ed := &ErrorDef{NamePos: NamePos(ped.NamePos), ID: verror.ID(pkg.Path + "." + name)}
			ed.Action = defineErrorAction(name, ped.Actions, file, env)
			ed.Params = defineErrorParams(name, ped.Params, file, env)
			ed.Formats = defineErrorFormats(name, ped.Formats, ed.Params, file, env)
			// We require the "en" base language for at least one of the Formats, and
			// favor "en-US" if it exists.  This requirement is an attempt to ensure
			// there is at least one common language across all errors.
			for _, lf := range ed.Formats {
				if lf.Lang == i18n.LangID("en-US") {
					ed.English = lf.Fmt
					break
				}
				if ed.English == "" && i18n.BaseLangID(lf.Lang) == i18n.LangID("en") {
					ed.English = lf.Fmt
				}
			}
			if ed.English == "" {
				env.Errorf(file, ed.Pos, "error %s invalid (must define at least one English format)", name)
				continue
			}
			file.ErrorDefs = append(file.ErrorDefs, ed)
		}
	}
}

func defineErrorAction(name string, pactions []parse.StringPos, file *File, env *Env) verror2.ActionCode {
	// We allow multiple actions to be specified in the parser, so that it's easy
	// to add new actions in the future.
	var act verror2.ActionCode
	seenRetry := false
	for _, pact := range pactions {
		if code, ok := parseRetryAction(pact.String); ok {
			if seenRetry {
				env.Errorf(file, pact.Pos, "error %s action %s invalid (retry action specified multiple times)", name, pact.String)
				continue
			}
			seenRetry = true
			act |= code
			continue
		}
		env.Errorf(file, pact.Pos, "error %s action %s invalid (unknown action)", name, pact.String)
	}
	return act
}

// TODO(toddw): Define verror2.ActionCode as a VDL enum, and remove this function.
func parseRetryAction(str string) (verror2.ActionCode, bool) {
	switch str {
	case "NoRetry":
		return verror2.NoRetry, true
	case "RetryConnection":
		return verror2.RetryConnection, true
	case "RetryRefetch":
		return verror2.RetryRefetch, true
	case "RetryBackoff":
		return verror2.RetryBackoff, true
	}
	return verror2.ActionCode(0), false
}

func defineErrorParams(name string, pparams []*parse.Field, file *File, env *Env) []*Arg {
	var params []*Arg
	seen := make(map[string]*parse.Field)
	for _, pparam := range pparams {
		pname, pos := pparam.Name, pparam.Pos
		if pname == "" {
			env.Errorf(file, pos, "error %s invalid (parameters must be named)", name)
			return nil
		}
		if dup := seen[pname]; dup != nil {
			env.Errorf(file, pos, "error %s param %s duplicate name (previous at %s)", name, pname, dup.Pos)
			continue
		}
		seen[pname] = pparam
		if _, err := ValidIdent(pname, ReservedCamelCase); err != nil {
			env.prefixErrorf(file, pos, err, "error %s param %s invalid", name, pname)
			continue
		}
		param := &Arg{NamePos(pparam.NamePos), compileType(pparam.Type, file, env)}
		params = append(params, param)
	}
	return params
}

func defineErrorFormats(name string, plfs []parse.LangFmt, params []*Arg, file *File, env *Env) []LangFmt {
	var lfs []LangFmt
	seen := make(map[i18n.LangID]parse.LangFmt)
	for _, plf := range plfs {
		pos, lang, fmt := plf.Pos(), i18n.LangID(plf.Lang.String), plf.Fmt.String
		if lang == "" {
			env.Errorf(file, pos, "error %s has empty language identifier", name)
			continue
		}
		if dup, ok := seen[lang]; ok {
			env.Errorf(file, pos, "error %s duplicate language %s (previous at %s)", name, lang, dup.Pos())
			continue
		}
		seen[lang] = plf
		xfmt, err := xlateErrorFormat(fmt, params)
		if err != nil {
			env.prefixErrorf(file, pos, err, "error %s language %s format invalid", name, lang)
			continue
		}
		lfs = append(lfs, LangFmt{lang, xfmt})
	}
	return lfs
}

// xlateErrorFormat translates the user-supplied format into the format
// expected by i18n, mainly translating parameter names into numeric indexes.
func xlateErrorFormat(format string, params []*Arg) (string, error) {
	const prefix = "{1:}{2:}"
	if format == "" {
		return prefix, nil
	}
	// Create a map from param name to index.  The index numbering starts at 3,
	// since the first two params are the component and op name, and i18n formats
	// use 1-based indices.
	pmap := make(map[string]string)
	for ix, param := range params {
		pmap[param.Name] = strconv.Itoa(ix + 3)
	}
	tagRE, err := regexp.Compile(`\{\:?([0-9a-zA-Z_]+)\:?\}`)
	if err != nil {
		return "", err
	}
	result, pos := prefix+" ", 0
	for _, match := range tagRE.FindAllStringSubmatchIndex(format, -1) {
		// The tag submatch indices are available as match[2], match[3]
		if len(match) != 4 || match[2] < pos || match[2] > match[3] {
			return "", fmt.Errorf("internal error: bad regexp indices %v", match)
		}
		beg, end := match[2], match[3]
		tag := format[beg:end]
		if tag == "_" {
			continue // Skip underscore tags.
		}
		if _, err := strconv.Atoi(tag); err == nil {
			continue // Skip number tags.
		}
		xtag, ok := pmap[tag]
		if !ok {
			return "", fmt.Errorf("unknown param %q", tag)
		}
		// Replace tag with xtag in the result.
		result += format[pos:beg]
		result += xtag
		pos = end
	}
	if end := len(format); pos < end {
		result += format[pos:end]
	}
	return result, nil
}
