package gmaps

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gosom/scrapemate"
	"github.com/playwright-community/playwright-go"

	"github.com/gosom/google-maps-scraper/exiter"
)

// Heuristic parser for Google Maps dump blocks
func extractBusinessInfo(data any) (name, address, phone, category string) {
	var (
		possibleNames      []string
		possibleAddresses  []string
		possiblePhones     []string
		possibleCategories []string
	)

	var walk func(any)
	walk = func(v any) {
		switch vv := v.(type) {
		case string:
			str := strings.TrimSpace(vv)
			if len(str) == 0 {
				return
			}
			// Ignorar unicode de ícone, textos genéricos e strings irrelevantes
			// Corrigir filtro unicode: string(rune(0xe0c8)) == "\ue0c8"
			isIconUnicode := strings.HasPrefix(str, string(rune(0xe0c8))) || strings.HasPrefix(str, string(rune(0xe0b0)))
			if len(str) < 3 || str == "Pesquisar" || isIconUnicode || strings.HasPrefix(str, "★") || strings.HasPrefix(str, "☆") {
				return
			}
			// Heurística para nome: evitar códigos, ícones, strings genéricas, pegar nomes compostos e evitar strings com muitos símbolos
			if len(str) > 2 && len(str) < 80 &&
				!strings.Contains(str, "@") &&
				!strings.Contains(str, "+") &&
				!strings.Contains(str, "http") &&
				!strings.Contains(str, "(") &&
				!strings.Contains(str, ")") &&
				!strings.Contains(str, ".com") &&
				!strings.Contains(str, "Telefone") &&
				!strings.Contains(str, "CEP") &&
				!strings.Contains(str, "Pesquisar") &&
				!strings.HasPrefix(str, "0x") &&
				!strings.HasPrefix(str, "sc2") &&
				!strings.HasPrefix(str, "spotlit") &&
				!strings.HasPrefix(str, "★") &&
				!strings.HasPrefix(str, "☆") &&
				!isIconUnicode &&
				!strings.ContainsAny(str, "0123456789") {
				possibleNames = append(possibleNames, str)
			}
			// Heurística para endereço: expandida para mais padrões brasileiros
			estados := []string{"- MG", "- SP", "- RJ", "- BA", "- RS", "- PR", "- SC", "- GO", "- PE", "- ES", "- CE", "- PA", "- AM", "- PB", "- RN", "- PI", "- AL", "- MT", "- MS", "- DF", "- SE", "- RO", "- TO", "- AC", "- AP", "- RR"}
			enderecos := []string{"Rua", "Av.", "Avenida", "Centro", "CEP", "Travessa", "Praça", "Rodovia", "Estrada", "Alameda", "Loteamento", "Bairro", "Condomínio", "Vila", "Distrito", "Quadra", "Bloco", "Edifício", "Galeria"}
			isEndereco := false
			for _, uf := range estados {
				if strings.Contains(str, uf) {
					isEndereco = true
					break
				}
			}
			if !isEndereco {
				for _, termo := range enderecos {
					if strings.Contains(str, termo) {
						isEndereco = true
						break
					}
				}
			}
			if isEndereco && len(str) > 10 && !isIconUnicode && !strings.HasPrefix(str, "★") && !strings.HasPrefix(str, "☆") {
				possibleAddresses = append(possibleAddresses, str)
			}
			// Heurística para telefone: evitar unicode, pegar padrões brasileiros
			if (strings.HasPrefix(str, "(") && strings.Contains(str, ") ")) || strings.HasPrefix(str, "+") {
				if !isIconUnicode {
					possiblePhones = append(possiblePhones, str)
				}
			}
			// Heurística para categoria: expandida para evitar termos de endereço, cidades, estados, telefones, nomes, etc.
			lower := strings.ToLower(str)
			cidades := []string{"divinópolis", "são paulo", "rio de janeiro", "salvador", "porto alegre", "curitiba", "belo horizonte", "fortaleza", "manaus", "recife", "goiânia", "belém", "campinas", "são luís", "maceió", "duque de caxias", "natal", "teresina", "são gonçalo", "joão pessoa", "cuiabá", "campo grande", "são bernardo", "nova iguaçu", "santo andré", "osasco", "são josé dos campos", "jaboatão", "são josé do rio preto", "ribeirão preto", "uberlândia", "londrina", "joinville", "juiz de fora", "aparecida de goiânia", "anapolis", "serra", "sorocaba", "niterói", "caxias do sul", "florianópolis", "vila velha", "mauá", "carapicuíba", "santos", "guarulhos", "barueri", "são vicente", "cariacica", "caucaia", "itabuna", "ilhéus", "aracaju", "palmas", "macapá", "rio branco", "boa vista"}
			isCidade := false
			for _, cidade := range cidades {
				if strings.Contains(lower, cidade) {
					isCidade = true
					break
				}
			}
			isEstado := false
			for _, uf := range estados {
				if strings.Contains(lower, strings.ToLower(uf[2:])) { // "mg", "sp", etc
					isEstado = true
					break
				}
			}
			if len(str) > 2 && len(str) < 50 &&
				!strings.ContainsAny(lower, "0123456789@+") &&
				!strings.Contains(lower, "http") &&
				!isCidade &&
				!isEstado &&
				!strings.Contains(lower, "rua") &&
				!strings.Contains(lower, "av.") &&
				!strings.Contains(lower, "avenida") &&
				!strings.Contains(lower, "cep") &&
				!strings.Contains(lower, "telefone") &&
				!strings.Contains(lower, "centro") &&
				!strings.Contains(lower, ".com") &&
				!strings.HasPrefix(str, "(") &&
				!strings.HasPrefix(str, "+") &&
				!isIconUnicode &&
				str != "Pesquisar" &&
				!strings.HasPrefix(str, "★") &&
				!strings.HasPrefix(str, "☆") {
				possibleCategories = append(possibleCategories, str)
			}
		case []any:
			for _, item := range vv {
				walk(item)
			}
		case map[string]any:
			for _, item := range vv {
				walk(item)
			}
		}
	}
	walk(data)

	if len(possibleNames) > 0 {
		name = possibleNames[0]
	}
	if len(possibleAddresses) > 0 {
		address = possibleAddresses[0]
	}
	if len(possiblePhones) > 0 {
		phone = possiblePhones[0]
	}
	if len(possibleCategories) > 0 {
		category = possibleCategories[0]
	}
	return
}

type PlaceJobOptions func(*PlaceJob)

type PlaceJob struct {
	scrapemate.Job

	UsageInResultststs  bool
	ExtractEmail        bool
	ExitMonitor         exiter.Exiter
	ExtractExtraReviews bool
}

// Extrai campos diretamente do HTML renderizado usando Playwright
func extractFromHTML(page playwright.Page) (name, address, phone, category string, err error) {
	// Nome
	nameJS := `(() => {
		let el = document.querySelector('[data-attrid="title"] span')
			|| document.querySelector('h1 span')
			|| document.querySelector('h1');
		if (!el) {
			// fallback: busca por strong ou span grande
			el = Array.from(document.querySelectorAll('span,strong')).find(e => e.innerText && e.innerText.length > 3 && e.offsetHeight > 20);
		}
		return el ? el.innerText : '';
	})()`
	nameI, err := page.Evaluate(nameJS)
	if err == nil {
		name, _ = nameI.(string)
	}

	// Endereço
	addressJS := `(() => {
		let el = document.querySelector('[data-item-id="address"] span')
			|| document.querySelector('[data-attrid*="address"] span')
			|| document.querySelector('button[data-item-id^="address:"] span');
		if (!el) {
			// fallback: busca por spans com padrão de endereço
			el = Array.from(document.querySelectorAll('span')).find(e => /\d{2,}.*[A-Za-z]/.test(e.innerText) && e.innerText.length > 10);
		}
		return el ? el.innerText : '';
	})()`
	addressI, err := page.Evaluate(addressJS)
	if err == nil {
		address, _ = addressI.(string)
	}

	// Telefone
	phoneJS := `(() => {
		let el = document.querySelector('[data-attrid*="phone"] span')
			|| document.querySelector('button[data-item-id^="phone:"] span');
		if (!el) {
			// fallback: busca por spans com padrão de telefone
			el = Array.from(document.querySelectorAll('span')).find(e => /\(\d{2}\) ?\d/.test(e.innerText) || /^\+\d+/.test(e.innerText));
		}
		return el ? el.innerText : '';
	})()`
	phoneI, err := page.Evaluate(phoneJS)
	if err == nil {
		phone, _ = phoneI.(string)
	}

	// Categoria
	catJS := `(() => {
		let el = document.querySelector('[data-attrid="subtitle"] span')
			|| document.querySelector('[data-attrid*="business_category"] span')
			|| document.querySelector('[data-attrid*="category"] span');
		if (!el) {
			// fallback: busca por spans com texto curto e sem números
			el = Array.from(document.querySelectorAll('span')).find(e => e.innerText && e.innerText.length > 2 && e.innerText.length < 50 && !/\d/.test(e.innerText));
		}
		return el ? el.innerText : '';
	})()`
	catI, err := page.Evaluate(catJS)
	if err == nil {
		category, _ = catI.(string)
	}

	return name, address, phone, category, nil

}

func WithPlaceJobExitMonitor(exitMonitor exiter.Exiter) PlaceJobOptions {
	return func(j *PlaceJob) {
		j.ExitMonitor = exitMonitor
	}
}

func (j *PlaceJob) Process(_ context.Context, resp *scrapemate.Response) (any, []scrapemate.IJob, error) {
	defer func() {
		resp.Document = nil
		resp.Body = nil
		resp.Meta = nil
	}()

	raw, ok := resp.Meta["json"].([]byte)
	if !ok {
		return nil, nil, fmt.Errorf("could not convert to []byte")
	}

	entry, err := EntryFromJSON(raw)
	if err != nil {
		return nil, nil, err
	}

	entry.ID = j.ParentID

	if entry.Link == "" {
		entry.Link = j.GetURL()
	}

	allReviewsRaw, ok := resp.Meta["reviews_raw"].(fetchReviewsResponse)
	if ok && len(allReviewsRaw.pages) > 0 {
		entry.AddExtraReviews(allReviewsRaw.pages)
	}

	if j.ExtractEmail && entry.IsWebsiteValidForEmail() {
		opts := []EmailExtractJobOptions{}
		if j.ExitMonitor != nil {
			opts = append(opts, WithEmailJobExitMonitor(j.ExitMonitor))
		}

		emailJob := NewEmailJob(j.ID, &entry, opts...)

		j.UsageInResultststs = false

		return nil, []scrapemate.IJob{emailJob}, nil
	} else if j.ExitMonitor != nil {
		j.ExitMonitor.IncrPlacesCompleted(1)
	}

	return &entry, nil, err
}

func (j *PlaceJob) BrowserActions(ctx context.Context, page playwright.Page) scrapemate.Response {
	var resp scrapemate.Response

	pageResponse, err := page.Goto(j.GetURL(), playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	})
	if err != nil {
		resp.Error = err

		return resp
	}

	if err = clickRejectCookiesIfRequired(page); err != nil {
		resp.Error = err

		return resp
	}

	const defaultTimeout = 5000

	err = page.WaitForURL(page.URL(), playwright.PageWaitForURLOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(defaultTimeout),
	})
	if err != nil {
		resp.Error = err

		return resp
	}

	resp.URL = pageResponse.URL()
	resp.StatusCode = pageResponse.Status()
	resp.Headers = make(http.Header, len(pageResponse.Headers()))

	for k, v := range pageResponse.Headers() {
		resp.Headers.Add(k, v)
	}

	raw, err := j.extractJSON(page)
	if err != nil {
		resp.Error = err

		return resp
	}

	if resp.Meta == nil {
		resp.Meta = make(map[string]any)
	}

	resp.Meta["json"] = raw

	if j.ExtractExtraReviews {
		reviewCount := j.getReviewCount(raw)
		if reviewCount > 8 { // we have more reviews
			params := fetchReviewsParams{
				page:        page,
				mapURL:      page.URL(),
				reviewCount: reviewCount,
			}

			reviewFetcher := newReviewFetcher(params)

			reviewData, err := reviewFetcher.fetch(ctx)
			if err != nil {
				return resp
			}

			resp.Meta["reviews_raw"] = reviewData
		}
	}

	return resp
}

func (j *PlaceJob) extractJSON(page playwright.Page) ([]byte, error) {
	rawI, err := page.Evaluate(js)
	if err != nil {
		return nil, err
	}

	raw, ok := rawI.(string)
	if !ok || strings.TrimSpace(raw) == "" {
		fmt.Printf("[gmaps/place.go] Nenhum dado retornado pelo JS para a URL: %s\n", page.URL())
		return nil, fmt.Errorf("no data returned by JS")
	}

	// Salvar o dump para análise (desativado por padrão)
	// fname := fmt.Sprintf("/gmapsdata/dump_%d.json", time.Now().UnixNano())
	// err = os.WriteFile(fname, []byte(raw), 0644)
	// if err != nil {
	// 	fmt.Printf("[gmaps/place.go] Erro ao salvar dump: %v\n", err)
	// } else {
	// 	fmt.Printf("[gmaps/place.go] Dump salvo em: %s\n", fname)
	// }

	// Novo parser: tentar encontrar os principais campos em todos os índices
	// do APP_INITIALIZATION_STATE
	var dump map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &dump); err != nil {
		fmt.Printf("[gmaps/place.go] Erro ao fazer unmarshal do dump: %v\n", err)
		return []byte(raw), nil // fallback para análise manual
	}

	// Extração priorizando o HTML
	htmlName, htmlAddress, htmlPhone, htmlCategory, htmlErr := extractFromHTML(page)
	if htmlErr == nil {
		fmt.Printf("[HTML PARSER] name=%q address=%q phone=%q category=%q\n", htmlName, htmlAddress, htmlPhone, htmlCategory)
	}
	// Fallback: heurística nos dumps, apenas para validação extra
	if ais, ok := dump["APP_INITIALIZATION_STATE"].([]interface{}); ok {
		for idx, item := range ais {
			b, _ := json.Marshal(item)
			fname := fmt.Sprintf("/gmapsdata/ais_idx%d_%d.json", idx, time.Now().UnixNano())
			os.WriteFile(fname, b, 0644)
			var block any
			if err := json.Unmarshal(b, &block); err == nil {
				hName, hAddress, hPhone, hCategory := extractBusinessInfo(block)
				fmt.Printf("[HEURISTIC PARSER] name=%q address=%q phone=%q category=%q\n", hName, hAddress, hPhone, hCategory)
				// Loga diferença, mas sempre prioriza o HTML
				finalName := htmlName
				finalAddress := htmlAddress
				finalPhone := htmlPhone
				finalCategory := htmlCategory
				if finalName == "" {
					finalName = hName
				}
				if finalAddress == "" {
					finalAddress = hAddress
				}
				if finalPhone == "" {
					finalPhone = hPhone
				}
				if finalCategory == "" {
					finalCategory = hCategory
				}
				fmt.Printf("[FINAL FIELDS] name=%q address=%q phone=%q category=%q\n", finalName, finalAddress, finalPhone, finalCategory)
			}
		}
	}

	// Retornar o dump original para não quebrar o fluxo
	return []byte(raw), nil
}

func (j *PlaceJob) getReviewCount(data []byte) int {
	tmpEntry, err := EntryFromJSON(data, true)
	if err != nil {
		return 0
	}

	return tmpEntry.ReviewCount
}

func (j *PlaceJob) UseInResults() bool {
	return j.UsageInResultststs
}

func ctxWait(ctx context.Context, dur time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(dur):
	}
}

const js = `
(function() {
	let result = {};
	if (window.APP_INITIALIZATION_STATE) {
		result['APP_INITIALIZATION_STATE'] = window.APP_INITIALIZATION_STATE;
	}
	if (window._pageData) {
		result['_pageData'] = window._pageData;
	}
	// Extraia todos os scripts JSON-LD
	let ldjson = [];
	document.querySelectorAll('script[type="application/ld+json"]').forEach(s => {
		ldjson.push(s.innerText);
	});
	result['ldjson'] = ldjson;
	return JSON.stringify(result);
})()
`
