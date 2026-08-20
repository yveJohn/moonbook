package modules_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

type dataOwner string

const (
	readerOwner   dataOwner = "reader"
	commerceOwner dataOwner = "commerce"
	novelOwner    dataOwner = "novel"
)

var tableOwners = map[string]dataOwner{
	"reader_accounts":                   readerOwner,
	"reader_sessions":                   readerOwner,
	"reader_invite_codes":               readerOwner,
	"reader_invite_relations":           readerOwner,
	"reader_bookshelf_entries":          readerOwner,
	"reader_book_likes":                 readerOwner,
	"reader_reading_history":            readerOwner,
	"reader_reading_preferences":        readerOwner,
	"reader_feedback":                   readerOwner,
	"reader_daily_activity":             readerOwner,
	"reader_activity_settings":          readerOwner,
	"commerce_products":                 commerceOwner,
	"commerce_chapter_pricing_config":   commerceOwner,
	"commerce_membership_grants":        commerceOwner,
	"commerce_entitlements":             commerceOwner,
	"commerce_reader_search_projection": commerceOwner,
	"reader_wallets":                    commerceOwner,
	"reader_wallet_ledgers":             commerceOwner,
	"reader_recharge_products":          commerceOwner,
	"reader_recharge_settings":          commerceOwner,
	"reader_payment_channels":           commerceOwner,
	"reader_recharge_orders":            commerceOwner,
	"reader_payment_callback_logs":      commerceOwner,
	"reader_checkin_reward_rules":       commerceOwner,
	"reader_checkin_records":            commerceOwner,
	"reader_purchase_orders":            commerceOwner,
	"reader_invite_reward_config":       commerceOwner,
	"reader_bonus_coin_buckets":         commerceOwner,
	"reader_invite_reward_records":      commerceOwner,
	"reader_wallet_adjustments":         commerceOwner,
	"novel_categories":                  novelOwner,
	"novel_authors":                     novelOwner,
	"novel_books":                       novelOwner,
	"novel_book_sub_categories":         novelOwner,
	"novel_book_tags":                   novelOwner,
	"novel_chapters":                    novelOwner,
	"novel_objects":                     novelOwner,
	"novel_object_references":           novelOwner,
	"novel_object_events":               novelOwner,
	"novel_reader_seo_config":           novelOwner,
	"novel_crawl_forum_source":          novelOwner,
	"novel_crawl_forum_board":           novelOwner,
	"novel_crawl_thread_candidate":      novelOwner,
	"novel_crawl_import_task":           novelOwner,
	"novel_crawl_fetch_log":             novelOwner,
	"novel_txt_import_task":             novelOwner,
	"novel_book_merge_task":             novelOwner,
	"novel_book_merge_source":           novelOwner,
	"novel_book_merge_chapter":          novelOwner,
	"novel_ai_config":                   novelOwner,
	"novel_ai_config_model":             novelOwner,
	"novel_chapter_clean_config":        novelOwner,
	"novel_chapter_clean_task":          novelOwner,
	"novel_chapter_clean_result":        novelOwner,
	"novel_chapter_summary_config":      novelOwner,
	"novel_chapter_summary_task":        novelOwner,
	"novel_book_profile_config":         novelOwner,
	"novel_book_profile_suggestion":     novelOwner,
}

func TestSQLOwnership(t *testing.T) {
	walkRuntimeModuleFiles(t, func(module, path string, file *ast.File) {
		constants := packageStringConstants(file)
		ast.Inspect(file, func(node ast.Node) bool {
			expression, ok := node.(ast.Expr)
			if !ok {
				return true
			}
			value, ok := staticString(expression, constants)
			if !ok {
				return true
			}
			for _, table := range referencedTables(value) {
				owner, registered := tableOwners[table]
				if !registered && hasDomainTablePrefix(table) {
					t.Errorf("%s: domain table %s is missing from the ownership registry", path, table)
					continue
				}
				if registered && string(owner) != module {
					t.Errorf("%s: module %s must not access %s owned by %s", path, module, table, tableOwners[table])
				}
			}
			return true
		})
	})
}

var tableReferencePattern = regexp.MustCompile(`(?i)\b(?:from|join|into|update)\s+([a-z_][a-z0-9_]*)`)

func referencedTables(sql string) []string {
	matches := tableReferencePattern.FindAllStringSubmatch(sql, -1)
	tables := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) == 2 {
			tables = append(tables, strings.ToLower(match[1]))
		}
	}
	return tables
}

func hasDomainTablePrefix(table string) bool {
	return strings.HasPrefix(table, "reader_") || strings.HasPrefix(table, "commerce_") || strings.HasPrefix(table, "novel_")
}

func packageStringConstants(file *ast.File) map[string]string {
	constants := map[string]string{}
	for _, declaration := range file.Decls {
		generic, ok := declaration.(*ast.GenDecl)
		if !ok || generic.Tok != token.CONST {
			continue
		}
		for _, spec := range generic.Specs {
			values, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for index, name := range values.Names {
				if index >= len(values.Values) {
					continue
				}
				if value, ok := staticString(values.Values[index], constants); ok {
					constants[name.Name] = value
				}
			}
		}
	}
	return constants
}

func staticString(expression ast.Expr, constants map[string]string) (string, bool) {
	switch value := expression.(type) {
	case *ast.BasicLit:
		if value.Kind != token.STRING {
			return "", false
		}
		text, err := strconv.Unquote(value.Value)
		return text, err == nil
	case *ast.Ident:
		text, ok := constants[value.Name]
		return text, ok
	case *ast.ParenExpr:
		return staticString(value.X, constants)
	case *ast.BinaryExpr:
		if value.Op != token.ADD {
			return "", false
		}
		left, leftOK := staticString(value.X, constants)
		right, rightOK := staticString(value.Y, constants)
		if !leftOK || !rightOK {
			return "", false
		}
		return left + right, true
	default:
		return "", false
	}
}

func TestCommerceProjectionUsageIsRestrictedToSearchAndDisplay(t *testing.T) {
	allowed := map[string]bool{
		filepath.Clean("commerce/readersearch/repository.go"):       true,
		filepath.Clean("commerce/adminoperations/repository.go"):    true,
		filepath.Clean("commerce/adminorder/repository.go"):         true,
		filepath.Clean("commerce/adminrechargeorder/repository.go"): true,
		filepath.Clean("commerce/adminwallet/repository.go"):        true,
	}
	pattern := regexp.MustCompile(`\bcommerce_reader_search_projection\b`)
	walkRuntimeModuleFiles(t, func(_ string, path string, file *ast.File) {
		used := false
		ast.Inspect(file, func(node ast.Node) bool {
			literal, ok := node.(*ast.BasicLit)
			if ok && literal.Kind == token.STRING && pattern.MatchString(literal.Value) {
				used = true
			}
			return true
		})
		if used && !allowed[filepath.Clean(path)] {
			t.Errorf("%s: Commerce Reader projection is limited to search synchronization and management display", path)
		}
	})
}

func TestSQLOwnershipRejectsDynamicTableNames(t *testing.T) {
	tablePrefix := regexp.MustCompile(`(?i)\b(?:from|join|into|update)\s*$`)
	walkRuntimeModuleFiles(t, func(_ string, path string, file *ast.File) {
		constants := packageStringConstants(file)
		ast.Inspect(file, func(node ast.Node) bool {
			binary, ok := node.(*ast.BinaryExpr)
			if !ok || binary.Op != token.ADD {
				return true
			}
			parts := flattenStringAddition(binary)
			for index := 0; index+1 < len(parts); index++ {
				prefix, staticPrefix := staticString(parts[index], constants)
				_, staticTable := staticString(parts[index+1], constants)
				if staticPrefix && tablePrefix.MatchString(prefix) && !staticTable {
					t.Errorf("%s: dynamic SQL table names are not auditable; use a static same-domain table", path)
				}
			}
			return true
		})
	})
}

func flattenStringAddition(expression ast.Expr) []ast.Expr {
	binary, ok := expression.(*ast.BinaryExpr)
	if !ok || binary.Op != token.ADD {
		return []ast.Expr{expression}
	}
	parts := flattenStringAddition(binary.X)
	return append(parts, flattenStringAddition(binary.Y)...)
}

func walkRuntimeModuleFiles(t *testing.T, visit func(module, path string, file *ast.File)) {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		module := entry.Name()
		err := filepath.WalkDir(module, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if parseErr != nil {
				return parseErr
			}
			visit(module, path, file)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
