package prediction

import (
	"fmt"
	"math"
	"regexp"
	"sync"

	"strings"

	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/query"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"gorm.io/gorm"
)

type index struct {
	Docs   map[string]map[string]int64 `json:"docs"`
	Tokens map[string]map[string]int64 `json:"tokens"`
}

type tfidfCache struct {
	sync.Once
	vector map[string]map[string]float64
	index  index
}

var cache tfidfCache

func loadVectorCache(db *gorm.DB) {
	postings := query.Init(db).All()
	idx := buldIndex(postings)

	cache.index = idx
	cache.vector = make(map[string]map[string]float64)

	for account := range idx.Docs {
		cache.vector[account] = tfidf(account, idx)
	}
}

func ClearCache() {
	cache = tfidfCache{}
}

func buldIndex(postings []posting.Posting) index {
	idx := index{
		Docs:   make(map[string]map[string]int64),
		Tokens: make(map[string]map[string]int64),
	}
	for _, p := range postings {
		if idx.Docs[p.Account] == nil {
			idx.Docs[p.Account] = make(map[string]int64)
		}
		for _, token := range tokenize(strings.Join([]string{strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", p.Amount.InexactFloat64()), "0"), "."), p.Payee}, " ")) {
			if idx.Tokens[token] == nil {
				idx.Tokens[token] = make(map[string]int64)
			}
			idx.Tokens[token][p.Account]++
			idx.Docs[p.Account][token]++
		}
	}
	return idx
}

func tfidf(account string, idx index) map[string]float64 {
	tfidf := make(map[string]float64)
	for token, freq := range idx.Docs[account] {
		tf := float64(freq) / float64(len(idx.Docs[account]))
		idf := math.Log(float64(len(idx.Docs))/(1+float64(len(idx.Tokens[token])))) + 1
		tfidf[token] = tf * idf
	}
	return tfidf
}

func tokenize(s string) []string {
	tokens := regexp.MustCompile("[ .()/:]+").Split(s, -1)
	tokens = lo.Map(tokens, func(s string, _ int) string {
		return strings.ToLower(s)
	})
	return lo.Filter(tokens, func(s string, _ int) bool {
		return strings.TrimSpace(s) != ""
	})
}

func GetTfIdf(db *gorm.DB) gin.H {
	cache.Do(func() {
		loadVectorCache(db)
	})
	return gin.H{"tf_idf": cache.vector, "index": cache.index}
}

// PredictAccount finds the best matching account for a query string, filtered by prefix.
// It mirrors the TypeScript predictAccount helper used in import templates.
func PredictAccount(db *gorm.DB, query string, prefix string) string {
	cache.Do(func() {
		loadVectorCache(db)
	})

	// Count token frequencies in the query
	tokens := tokenize(query)
	freqMap := make(map[string]int64)
	for _, t := range tokens {
		freqMap[t]++
	}
	uniqueCount := len(freqMap)
	if uniqueCount == 0 || len(cache.index.Docs) == 0 {
		return fallbackAccount(prefix)
	}

	// Compute query TF-IDF vector (same formula as TS: tf = count/unique_count)
	numDocs := len(cache.index.Docs)
	queryVector := make(map[string]float64)
	for token, count := range freqMap {
		tf := float64(count) / float64(uniqueCount)
		idf := math.Log(float64(numDocs)/(1+float64(len(cache.index.Tokens[token])))) + 1
		queryVector[token] = tf * idf
	}

	// Find best matching account by cosine similarity, filtered by prefix
	bestAccount := ""
	bestScore := -1.0
	for account, accountVector := range cache.vector {
		if !strings.HasPrefix(account, prefix) {
			continue
		}
		score := cosineSimilarity(queryVector, accountVector)
		if score > bestScore {
			bestScore = score
			bestAccount = account
		}
	}

	if bestAccount != "" {
		return bestAccount
	}
	return fallbackAccount(prefix)
}

func fallbackAccount(prefix string) string {
	if strings.HasSuffix(prefix, ":") {
		return prefix + "Unknown"
	}
	return prefix + ":Unknown"
}

func cosineSimilarity(q, a map[string]float64) float64 {
	var dot, magQ, magA float64
	for token, qv := range q {
		av := a[token]
		dot += qv * av
		magQ += qv * qv
		magA += av * av
	}
	// include tokens only in `a`
	for token, av := range a {
		if _, seen := q[token]; !seen {
			magA += av * av
		}
	}
	if magQ == 0 || magA == 0 {
		return 0
	}
	return dot / (math.Sqrt(magQ) * math.Sqrt(magA))
}
