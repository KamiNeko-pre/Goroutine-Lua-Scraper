package engine

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

const benchmarkStoryCount = 30

func benchmarkHNHTML() string {
	var body strings.Builder
	body.WriteString("<table>")
	for id := 1; id <= benchmarkStoryCount; id++ {
		fmt.Fprintf(&body, `<tr class="athing" id="%d"><td><span class="titleline"><a href="https://example.com/%d">Story %d</a><span class="sitestr">example.com</span></span></td></tr><tr><td class="subtext"><span class="subline"><span id="score_%d">%d points</span> by <a class="hnuser">author%d</a><a>ignore</a><a href="item?id=%d">%d comments</a></span></td></tr>`, id, id, id, id, id, id, id, id)
	}
	body.WriteString("</table>")
	return body.String()
}

func hnSelectors(id int) []string {
	row := fmt.Sprintf("tr[id='%d']", id)
	return []string{
		row + " .titleline > a",
		row + " .titleline > a",
		"#score_" + strconv.Itoa(id),
		row + " + tr .hnuser",
		row + " .sitestr",
		row + " + tr .subline > a:last-child",
	}
}

func BenchmarkHNRuleDOMQueriesFreshParse(b *testing.B) {
	html := benchmarkHNHTML()
	b.ReportAllocs()
	b.SetBytes(int64(len(html)))
	for b.Loop() {
		for id := 1; id <= benchmarkStoryCount; id++ {
			for _, selector := range hnSelectors(id) {
				doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
				if err != nil {
					b.Fatal(err)
				}
				_ = doc.Find(selector).First().Text()
			}
		}
	}
}

func BenchmarkHNRuleDOMQueriesTaskCache(b *testing.B) {
	html := benchmarkHNHTML()
	b.ReportAllocs()
	b.SetBytes(int64(len(html)))
	for b.Loop() {
		cache := newDOMCache()
		for id := 1; id <= benchmarkStoryCount; id++ {
			for _, selector := range hnSelectors(id) {
				doc, err := cache.document(html)
				if err != nil {
					b.Fatal(err)
				}
				_ = doc.Find(selector).First().Text()
			}
		}
	}
}
