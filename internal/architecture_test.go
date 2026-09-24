package internal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

const modulePrefix = "github.com/giovalgas/envault/"

const (
	kindBC       = "bc"
	kindShared   = "shared"
	kindDelivery = "delivery"
	kindCmd      = "cmd"
	kindTool     = "tool"
	kindOther    = "other"
	kindExternal = "external"
)

var (
	exceptionFrom = modulePrefix + "internal/compose/infra/vaultsource"
	exceptionTo   = modulePrefix + "internal/vault/usecase"
)

var forbiddenDeliveryImports = []string{
	modulePrefix + "internal/shared/dotenv",
	"os/exec",
}

var forbiddenCommandAndScreenImports = []string{
	"os",
	"io/fs",
	"path/filepath",
}

type listedPackage struct {
	ImportPath string
	Imports    []string
}

type classification struct {
	kind  string
	bc    string
	layer string
}

func classify(importPath string) classification {
	if !strings.HasPrefix(importPath, modulePrefix) {
		return classification{kind: kindExternal}
	}

	rel := strings.TrimPrefix(importPath, modulePrefix)
	parts := strings.Split(rel, "/")

	switch parts[0] {
	case "cmd":
		return classification{kind: kindCmd}
	case "tools":
		return classification{kind: kindTool}
	case "internal":
		if len(parts) < 2 {
			return classification{kind: kindOther}
		}
		switch parts[1] {
		case "shared":
			return classification{kind: kindShared}
		case "delivery":
			return classification{kind: kindDelivery}
		case "vault", "compose", "skill":
			layer := ""
			if len(parts) >= 3 {
				layer = parts[2]
			}
			return classification{kind: kindBC, bc: parts[1], layer: layer}
		}
	}

	return classification{kind: kindOther}
}

func modulePackages(t *testing.T) map[string]listedPackage {
	t.Helper()

	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go não encontrado no PATH, teste de arquitetura pulado")
	}

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("não foi possível localizar o diretório do módulo")
	}
	moduleRoot := filepath.Dir(filepath.Dir(thisFile))

	cmd := exec.Command(goBin, "list", "-deps", "-json", "./...")
	cmd.Dir = moduleRoot
	cmd.Env = append(cmd.Environ(), "GOTOOLCHAIN=local")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("go list -deps -json ./... falhou: %v\n%s", err, stderr.String())
	}

	packages := make(map[string]listedPackage)
	decoder := json.NewDecoder(&stdout)
	for {
		var pkg listedPackage
		decodeErr := decoder.Decode(&pkg)
		if errors.Is(decodeErr, io.EOF) {
			break
		}
		if decodeErr != nil {
			t.Fatalf("não foi possível decodificar a saída de go list: %v", decodeErr)
		}
		if strings.HasPrefix(pkg.ImportPath, modulePrefix) {
			packages[pkg.ImportPath] = pkg
		}
	}

	return packages
}

func checkDomainOnlyImportsShared(packages map[string]listedPackage) []string {
	var violations []string

	for path, pkg := range packages {
		from := classify(path)
		if from.kind != kindBC || from.layer != "domain" {
			continue
		}

		for _, imp := range pkg.Imports {
			to := classify(imp)
			if to.kind == kindExternal {
				continue
			}
			if to.kind != kindShared {
				violations = append(violations, fmt.Sprintf(
					"%s importa %s: domain só pode importar internal/shared/...", path, imp,
				))
			}
		}
	}

	return violations
}

func checkUsecaseOnlyOwnDomainAndShared(packages map[string]listedPackage) []string {
	var violations []string

	for path, pkg := range packages {
		from := classify(path)
		if from.kind != kindBC || from.layer != "usecase" {
			continue
		}

		for _, imp := range pkg.Imports {
			to := classify(imp)
			if to.kind == kindExternal {
				continue
			}

			allowed := to.kind == kindShared || (to.kind == kindBC && to.bc == from.bc && to.layer == "domain")
			if !allowed {
				violations = append(violations, fmt.Sprintf(
					"%s importa %s: usecase só pode importar o próprio domain e internal/shared/...", path, imp,
				))
			}
		}
	}

	return violations
}

func checkNoCrossBoundedContextImport(packages map[string]listedPackage) []string {
	var violations []string

	for path, pkg := range packages {
		from := classify(path)
		if from.kind != kindBC {
			continue
		}

		for _, imp := range pkg.Imports {
			to := classify(imp)
			if to.kind != kindBC || to.bc == from.bc {
				continue
			}

			if path == exceptionFrom && imp == exceptionTo {
				continue
			}

			violations = append(violations, fmt.Sprintf(
				"%s importa %s: nenhum BC importa domain, usecase ou infra de outro BC", path, imp,
			))
		}
	}

	return violations
}

func checkDeliveryProductionDoesNotImportInfra(packages map[string]listedPackage) []string {
	var violations []string

	for path, pkg := range packages {
		from := classify(path)
		if from.kind != kindDelivery {
			continue
		}

		for _, imp := range pkg.Imports {
			to := classify(imp)
			if to.kind == kindBC && (to.layer == "infra" || to.layer == "domain") {
				violations = append(violations, fmt.Sprintf(
					"%s importa %s: código de produção de delivery não importa domain nem infra de nenhum BC", path, imp,
				))
				continue
			}

			for _, forbidden := range forbiddenDeliveryImports {
				if imp == forbidden {
					violations = append(violations, fmt.Sprintf(
						"%s importa %s: código de produção de delivery não importa internal/shared/dotenv nem os/exec", path, imp,
					))
				}
			}
		}
	}

	return violations
}

func isCliCommandPackage(importPath string) bool {
	return strings.HasPrefix(importPath, modulePrefix+"internal/delivery/cli/command/")
}

func isTuiScreenPackage(importPath string) bool {
	return strings.HasPrefix(importPath, modulePrefix+"internal/delivery/tui/screen/")
}

func checkCommandAndScreenDoNotImportFilesystemPackages(packages map[string]listedPackage) []string {
	var violations []string

	for path, pkg := range packages {
		if !isCliCommandPackage(path) && !isTuiScreenPackage(path) {
			continue
		}

		for _, imp := range pkg.Imports {
			for _, forbidden := range forbiddenCommandAndScreenImports {
				if imp == forbidden {
					violations = append(violations, fmt.Sprintf(
						"%s importa %s: delivery/cli/command/... e delivery/tui/screen/... não importam os, io/fs nem path/filepath", path, imp,
					))
				}
			}
		}
	}

	return violations
}

func checkCliCommandDoesNotImportFmt(packages map[string]listedPackage) []string {
	var violations []string

	for path, pkg := range packages {
		if !isCliCommandPackage(path) {
			continue
		}

		for _, imp := range pkg.Imports {
			if imp == "fmt" {
				violations = append(violations, fmt.Sprintf(
					"%s importa %s: delivery/cli/command/... não escreve direto em stdout ou stderr, só pelo presenter", path, imp,
				))
			}
		}
	}

	return violations
}

func checkTuiScreenOnlyImportsThemeViewmodelAndUsecases(packages map[string]listedPackage) []string {
	var violations []string
	themePath := modulePrefix + "internal/delivery/tui/theme"
	viewmodelPath := modulePrefix + "internal/delivery/tui/viewmodel"

	for path, pkg := range packages {
		if !isTuiScreenPackage(path) {
			continue
		}

		for _, imp := range pkg.Imports {
			to := classify(imp)
			if to.kind == kindExternal {
				continue
			}
			if to.kind == kindBC && to.layer == "usecase" {
				continue
			}
			if to.kind == kindDelivery && (imp == themePath || imp == viewmodelPath) {
				continue
			}

			violations = append(violations, fmt.Sprintf(
				"%s importa %s: uma tela da TUI só importa theme, viewmodel e usecase dos BCs, dentro do módulo", path, imp,
			))
		}
	}

	return violations
}

func checkSharedDoesNotImportBoundedContextsOrDelivery(packages map[string]listedPackage) []string {
	var violations []string

	for path, pkg := range packages {
		from := classify(path)
		if from.kind != kindShared {
			continue
		}

		for _, imp := range pkg.Imports {
			to := classify(imp)
			if to.kind == kindBC || to.kind == kindDelivery {
				violations = append(violations, fmt.Sprintf(
					"%s importa %s: internal/shared/... não importa nenhum BC nem delivery", path, imp,
				))
			}
		}
	}

	return violations
}

func checkOnlyCmdEnvaultImportsDelivery(packages map[string]listedPackage) []string {
	var violations []string
	cmdPath := modulePrefix + "cmd/envault"

	for path, pkg := range packages {
		if path == cmdPath {
			continue
		}

		from := deliveryInterface(path)
		for _, imp := range pkg.Imports {
			if classify(imp).kind != kindDelivery {
				continue
			}
			to := deliveryInterface(imp)
			if from != "" && from == to {
				continue
			}
			violations = append(violations, fmt.Sprintf(
				"%s importa %s: só cmd/envault importa internal/delivery/..., e dentro da delivery só a mesma interface (cli ou tui)", path, imp,
			))
		}
	}

	return violations
}

func deliveryInterface(importPath string) string {
	if classify(importPath).kind != kindDelivery {
		return ""
	}
	parts := strings.Split(strings.TrimPrefix(importPath, modulePrefix), "/")
	if len(parts) < 3 {
		return ""
	}
	return parts[2]
}

func TestArchitectureDependencyRules(t *testing.T) {
	packages := modulePackages(t)

	var violations []string
	violations = append(violations, checkDomainOnlyImportsShared(packages)...)
	violations = append(violations, checkUsecaseOnlyOwnDomainAndShared(packages)...)
	violations = append(violations, checkNoCrossBoundedContextImport(packages)...)
	violations = append(violations, checkDeliveryProductionDoesNotImportInfra(packages)...)
	violations = append(violations, checkSharedDoesNotImportBoundedContextsOrDelivery(packages)...)
	violations = append(violations, checkOnlyCmdEnvaultImportsDelivery(packages)...)
	violations = append(violations, checkCommandAndScreenDoNotImportFilesystemPackages(packages)...)
	violations = append(violations, checkCliCommandDoesNotImportFmt(packages)...)
	violations = append(violations, checkTuiScreenOnlyImportsThemeViewmodelAndUsecases(packages)...)

	if len(violations) == 0 {
		return
	}

	sort.Strings(violations)
	t.Errorf("regra de dependência de arquitetura violada:\n%s", strings.Join(violations, "\n"))
}
