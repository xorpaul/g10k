package main

//go:generate goyacc -o puppetfile_parser.go -p pfyy puppetfile_parser.y

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type pfAttribute struct {
	Name        string
	Value       string
	HasArrow    bool
	IsBareValue bool
}

type pfParserState struct {
	pfPath           string
	source           string
	branch           string
	forceForgeVers   bool
	currentModuleDir string
	moduleDirs       []string
	puppetFile       Puppetfile
}

func newPFParserState(pfPath string, source string, branch string, forceForgeVersions bool) *pfParserState {
	moduleDir := "modules"
	if len(moduleDirParam) != 0 {
		moduleDir = moduleDirParam
	}
	return &pfParserState{
		pfPath:           pfPath,
		source:           source,
		branch:           branch,
		forceForgeVers:   forceForgeVersions,
		currentModuleDir: moduleDir,
		puppetFile: Puppetfile{
			forgeModules: map[string]ForgeModule{},
			gitModules:   map[string]GitModule{},
		},
	}
}

func (s *pfParserState) handleModuledir(value string) {
	if len(moduleDirParam) != 0 {
		return
	}
	s.currentModuleDir = normalizeDir(value)
	s.moduleDirs = append(s.moduleDirs, s.currentModuleDir)
}

func (s *pfParserState) handleForgeBaseURL(value string) {
	s.puppetFile.forgeBaseURL = value
}

func (s *pfParserState) handleForgeCacheTTL(value string, line string) {
	ttl, err := time.ParseDuration(value)
	if err != nil {
		Fatalf("Error: Can not convert value " + value + " of parameter forge.cacheTtl " + value + " to a golang Duration. Valid time units are 300ms, 1.5h or 2h45m. In " + s.pfPath + " line: " + line)
	}
	s.puppetFile.forgeCacheTTL = ttl
}

func (s *pfParserState) handleMod(moduleName string, attrs []pfAttribute, line string) {
	moduleName = strings.TrimSpace(moduleName)
	if strings.Contains(moduleName, "/") || strings.Contains(moduleName, "-") {
		s.handleForgeModule(moduleName, attrs, line)
		return
	}
	s.handleGitModule(moduleName, attrs, line)
}

func (s *pfParserState) handleForgeModule(moduleName string, attrs []pfAttribute, line string) {
	comp := strings.Split(moduleName, "/")
	sep := "/"
	if len(comp) != 2 {
		comp = strings.Split(moduleName, "-")
		sep = "-"
		if len(comp) != 2 {
			Fatalf("Error: Forge module name is invalid! Should be like puppetlabs/apt or puppetlabs-apt, but is: " + moduleName + " in " + s.pfPath + " line: " + line)
		}
	}
	author := comp[0]
	name := comp[1]

	if _, ok := s.puppetFile.forgeModules[name]; ok {
		Fatalf("Error: Duplicate forge module found in " + s.pfPath + " for module " + author + "/" + name + " line: " + line)
	}

	version := "present"
	sha256sum := ""
	isForgeNotationGit := false
	forgeGitURL := ""

	for _, a := range attrs {
		if a.IsBareValue {
			version = a.Value
			continue
		}
		if !a.HasArrow {
			if a.Name != "" {
				version = a.Name
			}
			continue
		}
		if a.Name == "sha256sum" {
			sha256sum = a.Value
			continue
		}
		if a.Name == "git" {
			isForgeNotationGit = true
			forgeGitURL = a.Value
			continue
		}
		version = a.Value
	}

	// Normalize version: if it starts with ':' (from a quoted symbol like ':latest'),
	// treat it as a symbol and strip the leading colon
	if strings.HasPrefix(version, ":") && !strings.Contains(version, ".") {
		version = version[1:]
	}

	if isForgeNotationGit {
		if _, ok := s.puppetFile.forgeModules[name]; ok {
			Fatalf("Error: Forge Puppet module with same name found in " + s.pfPath + " for module " + name + " line: " + line)
		}
		if _, ok := s.puppetFile.gitModules[name]; ok {
			Fatalf("Error: Duplicate module found in " + s.pfPath + " for module " + name + " line: " + line)
		}
		gm, _ := s.parseGitAttributes(name, attrs, line)
		if len(gm.git) == 0 {
			gm.git = forgeGitURL
		}
		gm.moduleDir = s.currentModuleDir
		s.puppetFile.gitModules[name] = gm
		_ = sep
		return
	}

	if s.forceForgeVers && (version == "present" || version == "latest") {
		Fatalf("Error: Found " + version + " setting for forge module in " + s.pfPath + " for module " + author + "/" + name + " line: " + line + " and force_forge_versions is set to true! Please specify a version (e.g. '2.3.0')")
	}
	if _, ok := s.puppetFile.gitModules[name]; ok {
		Fatalf("Error: Forge Puppet module with same name found in " + s.pfPath + " for module " + name + " line: " + line)
	}

	if len(s.puppetFile.forgeBaseURL) == 0 {
		s.puppetFile.forgeBaseURL = config.ForgeBaseURL
	}
	s.puppetFile.forgeModules[name] = ForgeModule{
		version:      version,
		name:         name,
		author:       author,
		sha256sum:    sha256sum,
		moduleDir:    s.currentModuleDir,
		sourceBranch: s.source + "_" + s.branch,
	}
}

func (s *pfParserState) handleGitModule(moduleName string, attrs []pfAttribute, line string) {
	if strings.Contains(moduleName, "-") {
		Warnf("Warning: Found invalid character '-' in Puppet module name " + moduleName + " in " + s.pfPath + " line: " + line + "\n See module guidelines: https://docs.puppet.com/puppet/latest/reference/lang_reserved.html#modules")
	}
	if len(attrs) == 0 {
		if dryRun {
			Fatalf("Error: Missing :git url in " + s.pfPath + " for module " + moduleName + " line: " + line)
		}
		return
	}
	if len(attrs) > 4 {
		Fatalf("Error: Too many attributes in " + s.pfPath + " for module " + moduleName + " line: " + line)
	}
	if _, ok := s.puppetFile.gitModules[moduleName]; ok {
		Fatalf("Error: Duplicate module found in " + s.pfPath + " for module " + moduleName + " line: " + line)
	}

	gm, hasGitOrLocalAttr := s.parseGitAttributes(moduleName, attrs, line)

	if len(gm.git) == 0 && !hasGitOrLocalAttr {
		Fatalf("Error: Missing :git url in " + s.pfPath + " for module " + moduleName + " line: " + line)
	}

	if _, ok := s.puppetFile.forgeModules[moduleName]; ok {
		Fatalf("Error: Git Puppet module with same name found in " + s.pfPath + " for module " + moduleName + " line: " + line)
	}
	if config.IgnoreUnreachableModules {
		Debugf("Setting :ignore_unreachable for Git module " + moduleName)
		gm.ignoreUnreachable = true
	}
	s.puppetFile.gitModules[moduleName] = gm
}

func (s *pfParserState) parseGitAttributes(moduleName string, attrs []pfAttribute, line string) (GitModule, bool) {
	gm := GitModule{moduleDir: s.currentModuleDir}
	seenConflicting := make([]string, 0, 3)
	seenKeys := make(map[string]bool)
	hasGitOrLocalAttr := false

	for _, a := range attrs {
		if !a.HasArrow {
			Fatalf("Error: Trailing comma or invalid setting for module found in " + s.pfPath + " for module " + moduleName + " line: " + line)
		}
		if seenKeys[a.Name] {
			Fatalf("Error: Duplicate module found in " + s.pfPath + " for module " + moduleName + " line: " + line)
		}
		seenKeys[a.Name] = true

		switch a.Name {
		case "git":
			hasGitOrLocalAttr = true
			if strings.Contains(a.Value, "ProxyCommand") {
				Fatalf("Error: Found ProxyCommand option in git url in " + s.pfPath + " for module " + moduleName + " line: " + line)
			}
			gm.git = a.Value
		case "branch":
			seenConflicting = append(seenConflicting, ":branch")
			if a.Value == ":control_branch" || a.Value == "control_branch" {
				gm.link = true
			} else {
				gm.branch = a.Value
			}
		case "tag":
			seenConflicting = append(seenConflicting, ":tag")
			gm.tag = a.Value
		case "commit":
			seenConflicting = append(seenConflicting, ":commit")
			gm.commit = a.Value
		case "ref":
			seenConflicting = append(seenConflicting, ":ref")
			gm.ref = a.Value
		case "install_path":
			gm.installPath = a.Value
		case "link":
			seenConflicting = append(seenConflicting, ":link")
			link, err := strconv.ParseBool(a.Value)
			if err != nil {
				Fatalf("Error: Can not convert value " + a.Value + " of parameter link to boolean. In " + s.pfPath + " for module " + moduleName + " line: " + line)
			}
			gm.link = link
		case "ignore-unreachable", "ignore_unreachable":
			ignoreUnreachable, err := strconv.ParseBool(a.Value)
			if err != nil {
				Fatalf("Error: Can not convert value " + a.Value + " of parameter " + a.Name + " to boolean. In " + s.pfPath + " for module " + moduleName + " line: " + line)
			}
			gm.ignoreUnreachable = ignoreUnreachable
		case "fallback", "default_branch":
			gm.fallback = make([]string, strings.Count(a.Value, "|")+1)
			for i, fallbackBranch := range strings.Split(a.Value, "|") {
				gm.fallback[i] = strings.TrimSpace(fallbackBranch)
			}
		case "local":
			hasGitOrLocalAttr = true
			local, err := strconv.ParseBool(a.Value)
			if err != nil {
				Fatalf("Error: Can not convert value " + a.Value + " of parameter local to boolean. In " + s.pfPath + " for module " + moduleName + " line: " + line)
			}
			gm.local = local
		case "use_ssh_agent":
			useSSHAgent, err := strconv.ParseBool(a.Value)
			if err != nil {
				Fatalf("Error: Can not convert value " + a.Value + " of parameter use_ssh_agent to boolean. In " + s.pfPath + " for module " + moduleName + " line: " + line)
			}
			gm.useSSHAgent = useSSHAgent
		default:
			Fatalf("Error: Trailing comma or invalid setting for module found in " + s.pfPath + " for module " + moduleName + " line: " + line)
		}
	}

	if len(seenConflicting) > 1 {
		Fatalf("Error: Found conflicting git attributes " + strings.Join(seenConflicting, ", ") + ", in " + s.pfPath + " for module " + moduleName + " line: " + line)
	}

	return gm, hasGitOrLocalAttr
}

func (s *pfParserState) finalize(sshKey string) Puppetfile {
	s.puppetFile.privateKey = sshKey
	s.puppetFile.source = s.source
	s.puppetFile.sourceBranch = s.branch
	if len(s.moduleDirs) == 0 {
		s.moduleDirs = append(s.moduleDirs, s.currentModuleDir)
	}
	s.puppetFile.moduleDirs = s.moduleDirs
	if len(s.puppetFile.forgeBaseURL) == 0 {
		s.puppetFile.forgeBaseURL = config.ForgeBaseURL
	}
	return s.puppetFile
}

type pfLexer struct {
	lines       []string
	lineIdx     int
	col         int
	currentLine string
	logicalLine string
	pfPath      string
	state       *pfParserState
}

func newPFLexer(input string, pfPath string, state *pfParserState) *pfLexer {
	return &pfLexer{lines: strings.Split(input, "\n"), pfPath: pfPath, state: state}
}

func (l *pfLexer) Error(_ string) {
	if len(strings.TrimSpace(l.currentLine)) == 0 {
		Fatalf("Error: Could not interpret Puppetfile " + l.pfPath)
	}
	if dryRun {
		Fatalf("Error: Could not interpret line: " + l.currentLine + " In " + l.pfPath)
	}
	Fatalf("Error: Trailing comma or invalid setting for module found in " + l.pfPath + " line: " + l.currentLine)
}

func (l *pfLexer) Lex(lval *pfyySymType) int {
	for {
		if l.lineIdx >= len(l.lines) {
			return 0
		}
		line := l.lines[l.lineIdx]
		l.currentLine = line
		if l.col >= len(line) {
			effectiveLine := strings.TrimSpace(stripPuppetComment(line))
			if len(effectiveLine) > 0 {
				l.logicalLine += effectiveLine
			}
			l.lineIdx++
			l.col = 0
			if len(effectiveLine) == 0 {
				continue
			}
			if strings.HasSuffix(effectiveLine, ",") {
				continue
			}
			if len(l.logicalLine) > 0 {
				l.currentLine = l.logicalLine
				l.logicalLine = ""
			}
			return NEWLINE
		}

		for l.col < len(line) && (line[l.col] == ' ' || line[l.col] == '\t') {
			l.col++
		}
		if l.col >= len(line) {
			continue
		}
		if line[l.col] == '#' {
			l.col = len(line)
			continue
		}

		if line[l.col] == ',' {
			l.col++
			return COMMA
		}
		if l.col+1 < len(line) && line[l.col] == '=' && line[l.col+1] == '>' {
			l.col += 2
			return ARROW
		}
		if line[l.col] == '\'' || line[l.col] == '"' {
			q := line[l.col]
			l.col++
			start := l.col
			for l.col < len(line) && line[l.col] != q {
				l.col++
			}
			if l.col >= len(line) {
				lval.str = line[start:]
				return STRING
			}
			lval.str = line[start:l.col]
			l.col++
			return STRING
		}
		if line[l.col] == ':' {
			l.col++
			start := l.col
			for l.col < len(line) && isSymbolChar(line[l.col]) {
				l.col++
			}
			lval.str = line[start:l.col]
			return SYMBOL
		}

		start := l.col
		for l.col < len(line) && !isDelimiter(line[l.col]) {
			l.col++
		}
		word := line[start:l.col]
		switch word {
		case "mod":
			return MOD
		case "moduledir":
			return MODULEDIR
		case "forge.baseUrl", "forge.baseURL":
			return FORGEBASEURL
		case "forge.cacheTtl", "forge.cacheTTL":
			return FORGECACHETTL
		default:
			lval.str = word
			return WORD
		}
	}
}

func stripPuppetComment(line string) string {
	if strings.Contains(line, "#") {
		return strings.Split(line, "#")[0]
	}
	return line
}

func isDelimiter(c byte) bool {
	return c == ' ' || c == '\t' || c == ',' || c == '\n'
}

func isSymbolChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-'
}

func ParsePuppetfile(pfPath string, sshKey string, source string, branch string, forceForgeVersions bool) Puppetfile {
	rawContent, err := os.ReadFile(pfPath)
	if err != nil {
		Fatalf("ParsePuppetfile(): Error while opening Puppetfile " + pfPath + " Error: " + err.Error())
	}
	content := string(rawContent)
	state := newPFParserState(pfPath, source, branch, forceForgeVersions)
	lexer := newPFLexer(content, pfPath, state)
	pfyyParse(lexer)
	return state.finalize(sshKey)
}
