package loader

import "strings"

var querySynonyms = map[string][]string{
	"oracle":     {"oracle", "оракл"},
	"оракл":      {"oracle", "оракл"},
	"tarantool":  {"tarantool", "тарантул"},
	"тарантул":   {"tarantool", "тарантул"},
	"postgresql": {"postgresql", "postgres", "постгря", "постгрес"},
	"postgres":   {"postgresql", "postgres", "постгря", "postgres"},
	"постгря":    {"postgresql", "postgres", "постгря", "постгрес"},
	"msgpack":    {"msgpack", "msgpuck"},
	"odbc":       {"odbc"},
	"jdbc":       {"jdbc"},
	"vshard":     {"vshard"},
	"cartridge":  {"cartridge", "картридж"},
	"vacuum":     {"vacuum", "вакуум"},
}

func ExpandQuery(query string) []string {
	tokens := Tokenize(query)
	if len(tokens) == 0 {
		return []string{query}
	}

	seen := map[string]struct{}{query: {}}
	queries := []string{query}

	var extra []string
	for _, token := range tokens {
		if syns, ok := querySynonyms[token]; ok {
			extra = append(extra, syns...)
		}
	}

	lower := strings.ToLower(query)
	if strings.Contains(lower, "oracle") || strings.Contains(lower, "оракл") {
		extra = append(extra, "odbc", "jdbc", "коннектор", "popen", "ora")
	}
	if strings.Contains(lower, "кластер") || strings.Contains(lower, "cluster") {
		extra = append(extra, "cartridge", "vshard", "replica", "consul", "etcd", "docker")
	}
	if strings.Contains(lower, "msgpack") {
		extra = append(extra, "msgpuck", "декодирован", "поле", "crud")
	}

	if len(extra) > 0 {
		expanded := query + " " + strings.Join(extra, " ")
		if _, ok := seen[expanded]; !ok {
			queries = append(queries, expanded)
			seen[expanded] = struct{}{}
		}
	}

	// Короткий вариант: ключевые термы без стоп-слов
	keyTerms := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if len([]rune(t)) >= 3 {
			keyTerms = append(keyTerms, t)
		}
	}
	if len(keyTerms) >= 2 {
		short := strings.Join(keyTerms, " ")
		if _, ok := seen[short]; !ok {
			queries = append(queries, short)
			seen[short] = struct{}{}
		}
	}

	return queries
}
