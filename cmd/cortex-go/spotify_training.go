package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
)

type spotifyTraining struct {
	Commands     []string               `json:"commands"`
	MusicWords   []string               `json:"musicWords"`
	RequestVerbs []string               `json:"requestVerbs"`
	Greetings    []string               `json:"greetings"`
	Fillers      []string               `json:"fillers"`
	Articles     []string               `json:"articles"`
	Genres       []spotifyTrainingGenre `json:"genres"`
}

type spotifyTrainingGenre struct {
	ID      string   `json:"id"`
	Spotify string   `json:"spotify"`
	Words   []string `json:"words"`
}

type spotifyParsedRequest struct {
	Query    string `json:"query"`
	Kind     string `json:"kind"`
	Original string `json:"original,omitempty"`
}

type spotifyParseResult struct {
	Request       *spotifyParsedRequest `json:"request,omitempty"`
	Recognized    bool                  `json:"recognized"`
	NeedsTarget   bool                  `json:"needsTarget"`
	Clarification string                `json:"clarification,omitempty"`
}

type spotifyLabPreview struct {
	Action      string `json:"action"`
	Kind        string `json:"kind"`
	Query       string `json:"query,omitempty"`
	Description string `json:"description"`
}

type spotifyWord struct {
	Original string
	Folded   string
}

var spotifyWordPattern = regexp.MustCompile(`[\p{L}\p{N}]+`)
var spotifyURLPattern = regexp.MustCompile(`(?i)https?://\S+`)
var spotifyMentionPattern = regexp.MustCompile(`@[\p{L}\p{N}_.]+`)
var spotifyCommandPattern = regexp.MustCompile(`^\s*(![a-zA-Z0-9_]+)\s*`)

func defaultSpotifyTraining() spotifyTraining {
	return spotifyTraining{
		Commands:     []string{"!playnow", "!tema", "!musica"},
		MusicWords:   []string{"tema", "temas", "temita", "temitas", "temon", "temones", "temazo", "temazos", "music", "musica", "musicas", "musicon", "musiquita", "cancion", "canciones", "cancioncita", "track", "tracks", "rola", "rolas", "rolita"},
		RequestVerbs: []string{"quiero escuchar", "quiero oir", "me pones", "me podes poner", "podes poner", "podrias poner", "poneme", "ponele", "ponete", "pones", "poner", "pongan", "pone", "pon", "pasame", "pasate", "pasale", "pasen", "pasa", "quiero", "quisiera", "queria", "pedime", "pedia", "pedi", "pedir", "pido", "necesito"},
		Greetings:    []string{"hola", "holis", "buen dia", "buenas tardes", "buenas noches", "buenas", "bue", "que tal", "q tal", "como va", "como andan", "como estas", "como te va", "epa", "hey", "saludos", "aloha"},
		Fillers:      []string{"che", "por favor", "porfa", "porfavor", "please", "dale", "gracias", "muchas gracias", "amigo", "amiga", "genio", "crack", "maestro", "capo", "grande", "bot", "porfi"},
		Articles:     []string{"un", "una", "unos", "unas", "el", "la", "los", "las", "algun", "alguna", "alguno", "algunos", "otro", "otra"},
		Genres: []spotifyTrainingGenre{
			{ID: "bachata", Spotify: "bachata", Words: []string{"bachata", "bachatas"}},
			{ID: "blues", Spotify: "blues", Words: []string{"blues"}},
			{ID: "chamame", Spotify: "chamame", Words: []string{"chamame"}},
			{ID: "cumbia", Spotify: "cumbia", Words: []string{"cumbia", "cumbias", "cumbia villera"}},
			{ID: "cuarteto", Spotify: "cuarteto", Words: []string{"cuarteto", "cuartetos"}},
			{ID: "electronica", Spotify: "electronic", Words: []string{"electronica", "electro", "edm"}},
			{ID: "folklore", Spotify: "folk", Words: []string{"folklore", "folclore"}},
			{ID: "funk", Spotify: "funk", Words: []string{"funk"}},
			{ID: "jazz", Spotify: "jazz", Words: []string{"jazz"}},
			{ID: "merengue", Spotify: "merengue", Words: []string{"merengue", "merengues"}},
			{ID: "metal", Spotify: "metal", Words: []string{"metal"}},
			{ID: "pop", Spotify: "pop", Words: []string{"pop"}},
			{ID: "rap", Spotify: "rap", Words: []string{"rap"}},
			{ID: "reggae", Spotify: "reggae", Words: []string{"reggae", "regae"}},
			{ID: "reggaeton", Spotify: "reggaeton", Words: []string{"reggaeton", "regueton", "regueto", "regeton", "regeaton", "regaeton"}},
			{ID: "romantico", Spotify: "romantico", Words: []string{"romantico", "romantica", "romanticos", "romanticas", "romanticon"}},
			{ID: "rock", Spotify: "rock", Words: []string{"rock"}},
			{ID: "salsa", Spotify: "salsa", Words: []string{"salsa", "salsas"}},
			{ID: "techno", Spotify: "techno", Words: []string{"techno", "tekno"}},
			{ID: "trap", Spotify: "trap", Words: []string{"trap", "traps"}},
			{ID: "vallenato", Spotify: "vallenato", Words: []string{"vallenato", "vallenatos"}},
		},
	}
}

func normalizeSpotifyTraining(input spotifyTraining) spotifyTraining {
	defaults := defaultSpotifyTraining()
	useDefault := func(value, fallback []string) []string {
		if value == nil {
			value = fallback
		}
		return normalizeSpotifyTrainingWords(value, 200)
	}
	if input.Genres == nil {
		input.Genres = defaults.Genres
	}
	genres := make([]spotifyTrainingGenre, 0, min(len(input.Genres), 100))
	for _, genre := range input.Genres[:min(len(input.Genres), 100)] {
		id := strings.TrimSpace(genre.ID)
		spotify := strings.TrimSpace(genre.Spotify)
		if id == "" {
			id = spotify
		}
		if spotify == "" {
			spotify = id
		}
		id = truncateRunes(id, 40)
		spotify = truncateRunes(spotify, 40)
		words := normalizeSpotifyTrainingWords(genre.Words, 60)
		if id != "" && spotify != "" && len(words) > 0 {
			genres = append(genres, spotifyTrainingGenre{ID: id, Spotify: spotify, Words: words})
		}
	}
	return spotifyTraining{
		Commands:     useDefault(input.Commands, defaults.Commands),
		MusicWords:   useDefault(input.MusicWords, defaults.MusicWords),
		RequestVerbs: useDefault(input.RequestVerbs, defaults.RequestVerbs),
		Greetings:    useDefault(input.Greetings, defaults.Greetings),
		Fillers:      useDefault(input.Fillers, defaults.Fillers),
		Articles:     useDefault(input.Articles, defaults.Articles),
		Genres:       genres,
	}
}

func normalizeSpotifyTrainingWords(words []string, limit int) []string {
	result := make([]string, 0, min(len(words), limit))
	seen := make(map[string]bool)
	for _, word := range words[:min(len(words), limit)] {
		clean := truncateRunes(strings.TrimSpace(word), 80)
		key := normalizeSpotifyMatchText(clean)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, clean)
	}
	return result
}

func (s *spotifyService) trainingSnapshot() spotifyTraining {
	s.mu.RLock()
	training := cloneSpotifyTraining(s.settings.Training)
	s.mu.RUnlock()
	return training
}

func cloneSpotifyTraining(training spotifyTraining) spotifyTraining {
	training.Commands = append([]string(nil), training.Commands...)
	training.MusicWords = append([]string(nil), training.MusicWords...)
	training.RequestVerbs = append([]string(nil), training.RequestVerbs...)
	training.Greetings = append([]string(nil), training.Greetings...)
	training.Fillers = append([]string(nil), training.Fillers...)
	training.Articles = append([]string(nil), training.Articles...)
	genres := make([]spotifyTrainingGenre, len(training.Genres))
	for index, genre := range training.Genres {
		genres[index] = genre
		genres[index].Words = append([]string(nil), genre.Words...)
	}
	training.Genres = genres
	return training
}

func (s *spotifyService) handleTraining(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, map[string]any{"training": s.trainingSnapshot(), "defaults": defaultSpotifyTraining()})
		return
	}
	if r.Method != http.MethodPut {
		w.Header().Set("Allow", "GET, PUT")
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 32768)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var input struct {
		Training spotifyTraining `json:"training"`
	}
	if err := decoder.Decode(&input); err != nil || decoder.Decode(&struct{}{}) != io.EOF || input.Training.MusicWords == nil || input.Training.RequestVerbs == nil {
		http.Error(w, "El vocabulario musical no es válido o está incompleto", http.StatusBadRequest)
		return
	}
	training := normalizeSpotifyTraining(input.Training)
	s.actionMu.Lock()
	defer s.actionMu.Unlock()
	s.mu.RLock()
	settings := s.settings
	s.mu.RUnlock()
	settings.Training = training
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if err := s.persistSettings(ctx, settings); err != nil {
		http.Error(w, "No se pudo guardar el vocabulario de Spotify", http.StatusServiceUnavailable)
		return
	}
	s.mu.Lock()
	s.settings = settings
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"training": cloneSpotifyTraining(training)})
}

func (s *spotifyService) handleTrainingReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}
	s.actionMu.Lock()
	defer s.actionMu.Unlock()
	s.mu.RLock()
	settings := s.settings
	s.mu.RUnlock()
	settings.Training = defaultSpotifyTraining()
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if err := s.persistSettings(ctx, settings); err != nil {
		http.Error(w, "No se pudo restaurar el vocabulario de Spotify", http.StatusServiceUnavailable)
		return
	}
	s.mu.Lock()
	s.settings = settings
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"training": cloneSpotifyTraining(settings.Training)})
}

func (s *spotifyService) handleTrainingPreview(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 2048)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var input struct {
		Comment string `json:"comment"`
	}
	if err := decoder.Decode(&input); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		http.Error(w, "El comentario de prueba no es válido", http.StatusBadRequest)
		return
	}
	input.Comment = strings.TrimSpace(input.Comment)
	if input.Comment == "" || len([]rune(input.Comment)) > 500 {
		http.Error(w, "Escribe un comentario de hasta 500 caracteres", http.StatusBadRequest)
		return
	}
	result := parseSpotifyTrainingRequest(input.Comment, s.trainingSnapshot())
	writeJSON(w, http.StatusOK, result)
}

func parseSpotifyTrainingRequest(raw string, vocabulary spotifyTraining) spotifyParseResult {
	original := strings.TrimSpace(spotifyURLPattern.ReplaceAllString(raw, " "))
	if original == "" {
		return spotifyParseResult{}
	}
	working := original
	command := ""
	if match := spotifyCommandPattern.FindStringSubmatch(working); len(match) > 1 {
		if containsSpotifyTrainingWord(vocabulary.Commands, match[1]) {
			command = match[1]
			working = strings.TrimSpace(working[len(match[0]):])
		} else {
			return spotifyParseResult{}
		}
	}
	working = spotifyMentionPattern.ReplaceAllString(working, " ")
	words := spotifyTrainingTokens(working)
	if len(words) == 0 {
		return spotifyParseResult{}
	}
	if words[0].Folded == "no" || words[0].Folded == "nunca" {
		return spotifyParseResult{}
	}

	stripLeadingTrainingPhrases(&words, vocabulary.Greetings)
	removeTrainingPhrases(&words, vocabulary.Fillers)
	starter := matchTrainingPrefix(words, vocabulary.RequestVerbs)
	if len(starter) > 0 {
		words = words[len(starter):]
	}
	hasMusicWord := containsTrainingPhrase(words, vocabulary.MusicWords)
	looseSignal := hasTrainingPrefix(words, []string{"algo", "un poco", "un poquito"})
	recognized := command != "" || len(starter) > 0 || hasMusicWord || looseSignal
	if !recognized {
		return spotifyParseResult{}
	}
	if len(words) == 0 {
		return spotifyParseResult{Recognized: true, NeedsTarget: true, Clarification: "¿Qué artista o canción querés escuchar?"}
	}

	cleaned := spotifyTrainingText(words)
	if hasMusicWord {
		candidate := stripSpotifyMusicPrefix(words, vocabulary)
		if left, right, ok := spotifySplitTrainingTrack(candidate); ok {
			title := spotifyTrainingText(stripSpotifyArtistArticles(left))
			artist := spotifyTrainingText(stripSpotifyArtistArticles(right))
			if title != "" && artist != "" {
				query := truncateRunes(title+" de "+artist, 160)
				return spotifyParseResult{Request: &spotifyParsedRequest{Query: query, Kind: "track", Original: original}, Recognized: true}
			}
		}
		for _, genre := range vocabulary.Genres {
			if containsSpotifyTrainingWord(genre.Words, spotifyTrainingText(candidate)) {
				request := &spotifyParsedRequest{Query: genre.Spotify, Kind: "genre", Original: original}
				return spotifyParseResult{Request: request, Recognized: true}
			}
		}
		artist := stripSpotifyArtistArticles(candidate)
		if len([]rune(normalizeSpotifyMatchText(spotifyTrainingText(artist)))) < 2 {
			return spotifyParseResult{Recognized: true, NeedsTarget: true, Clarification: "¿Qué artista o canción querés escuchar?"}
		}
		request := &spotifyParsedRequest{Query: truncateRunes(spotifyTrainingText(artist), 160), Kind: "artist", Original: original}
		return spotifyParseResult{Request: request, Recognized: true}
	}
	if looseSignal {
		candidate := dropTrainingPrefix(words, []string{"algo", "un poco", "un poquito"})
		candidate = dropTrainingPrefix(candidate, []string{"de", "del"})
		query := spotifyTrainingText(candidate)
		if len([]rune(normalizeSpotifyMatchText(query))) >= 2 {
			return spotifyParseResult{Request: &spotifyParsedRequest{Query: truncateRunes(query, 160), Kind: "artist", Original: original}, Recognized: true}
		}
		return spotifyParseResult{Recognized: true, NeedsTarget: true, Clarification: "¿Qué artista o canción querés escuchar?"}
	}

	if left, right, ok := spotifySplitTrainingTrack(words); ok {
		title := spotifyTrainingText(stripSpotifyArtistArticles(left))
		artist := spotifyTrainingText(stripSpotifyArtistArticles(right))
		if title != "" && artist != "" {
			query := truncateRunes(title+" de "+artist, 160)
			return spotifyParseResult{Request: &spotifyParsedRequest{Query: query, Kind: "track", Original: original}, Recognized: true}
		}
	}

	for _, genre := range vocabulary.Genres {
		if containsSpotifyTrainingWord(genre.Words, cleaned) {
			return spotifyParseResult{Request: &spotifyParsedRequest{Query: genre.Spotify, Kind: "genre", Original: original}, Recognized: true}
		}
	}

	candidate := stripSpotifyArtistArticles(words)
	candidate = stripSpotifyLeadingArticles(candidate, vocabulary.Articles)
	query := spotifyTrainingText(candidate)
	if len([]rune(normalizeSpotifyMatchText(query))) < 2 {
		return spotifyParseResult{Recognized: true, NeedsTarget: true, Clarification: "¿Qué artista o canción querés escuchar?"}
	}
	return spotifyParseResult{Request: &spotifyParsedRequest{Query: truncateRunes(query, 160), Kind: "artist", Original: original}, Recognized: true}
}

func spotifyTrainingTokens(value string) []spotifyWord {
	indices := spotifyWordPattern.FindAllStringIndex(value, -1)
	words := make([]spotifyWord, 0, len(indices))
	for _, index := range indices {
		original := value[index[0]:index[1]]
		folded := normalizeSpotifyMatchText(original)
		if folded != "" {
			words = append(words, spotifyWord{Original: original, Folded: folded})
		}
	}
	return words
}

func stripLeadingTrainingPhrases(words *[]spotifyWord, phrases []string) {
	for {
		matched := matchTrainingPrefix(*words, phrases)
		if len(matched) == 0 {
			return
		}
		*words = (*words)[len(matched):]
	}
}

func removeTrainingPhrases(words *[]spotifyWord, phrases []string) {
	for _, phrase := range sortTrainingPhrases(phrases) {
		pattern := trainingPhraseTokens(phrase)
		if len(pattern) == 0 {
			continue
		}
		filtered := make([]spotifyWord, 0, len(*words))
		for index := 0; index < len(*words); {
			if trainingPhraseAt(*words, pattern, index) {
				index += len(pattern)
				continue
			}
			filtered = append(filtered, (*words)[index])
			index++
		}
		*words = filtered
	}
}

func matchTrainingPrefix(words []spotifyWord, phrases []string) []spotifyWord {
	for _, phrase := range sortTrainingPhrases(phrases) {
		pattern := trainingPhraseTokens(phrase)
		if len(pattern) > 0 && trainingPhraseAt(words, pattern, 0) {
			return words[:len(pattern)]
		}
	}
	return nil
}

func hasTrainingPrefix(words []spotifyWord, phrases []string) bool {
	return len(matchTrainingPrefix(words, phrases)) > 0
}

func containsTrainingPhrase(words []spotifyWord, phrases []string) bool {
	for _, phrase := range sortTrainingPhrases(phrases) {
		pattern := trainingPhraseTokens(phrase)
		for index := 0; len(pattern) > 0 && index+len(pattern) <= len(words); index++ {
			if trainingPhraseAt(words, pattern, index) {
				return true
			}
		}
	}
	return false
}

func trainingPhraseAt(words []spotifyWord, phrase []string, index int) bool {
	if index < 0 || index+len(phrase) > len(words) {
		return false
	}
	for offset, expected := range phrase {
		if words[index+offset].Folded != expected {
			return false
		}
	}
	return true
}

func trainingPhraseTokens(phrase string) []string {
	words := spotifyTrainingTokens(phrase)
	result := make([]string, 0, len(words))
	for _, word := range words {
		result = append(result, word.Folded)
	}
	return result
}

func sortTrainingPhrases(phrases []string) []string {
	result := append([]string(nil), phrases...)
	sort.SliceStable(result, func(i, j int) bool {
		return len(trainingPhraseTokens(result[i])) > len(trainingPhraseTokens(result[j]))
	})
	return result
}

func containsSpotifyTrainingWord(words []string, target string) bool {
	targetWords := trainingPhraseTokens(target)
	if len(targetWords) == 0 {
		return false
	}
	for _, candidate := range words {
		candidateWords := trainingPhraseTokens(candidate)
		if len(candidateWords) == len(targetWords) {
			matches := true
			for index := range targetWords {
				if candidateWords[index] != targetWords[index] {
					matches = false
					break
				}
			}
			if matches {
				return true
			}
		}
	}
	return false
}

func stripSpotifyMusicPrefix(words []spotifyWord, vocabulary spotifyTraining) []spotifyWord {
	result := append([]spotifyWord(nil), words...)
	for pass := 0; pass < 3; pass++ {
		before := len(result)
		result = stripSpotifyArticleBeforeMusic(result, vocabulary)
		if prefix := matchTrainingPrefix(result, vocabulary.MusicWords); len(prefix) > 0 {
			result = result[len(prefix):]
		}
		result = dropTrainingPrefix(result, []string{"de", "del", "al"})
		if len(result) == before {
			break
		}
	}
	return result
}

func stripSpotifyArticleBeforeMusic(words []spotifyWord, vocabulary spotifyTraining) []spotifyWord {
	article := matchTrainingPrefix(words, vocabulary.Articles)
	if len(article) == 0 || len(article) == len(words) {
		return words
	}
	if music := matchTrainingPrefix(words[len(article):], vocabulary.MusicWords); len(music) > 0 {
		return words[len(article):]
	}
	return words
}

func stripSpotifyArtistArticles(words []spotifyWord) []spotifyWord {
	return dropTrainingPrefix(words, []string{"a", "de", "del", "al", "por"})
}

func stripSpotifyLeadingArticles(words []spotifyWord, articles []string) []spotifyWord {
	if prefix := matchTrainingPrefix(words, articles); len(prefix) > 0 {
		return words[len(prefix):]
	}
	return words
}

func dropTrainingPrefix(words []spotifyWord, phrases []string) []spotifyWord {
	for _, phrase := range sortTrainingPhrases(phrases) {
		pattern := trainingPhraseTokens(phrase)
		if len(pattern) > 0 && trainingPhraseAt(words, pattern, 0) {
			return words[len(pattern):]
		}
	}
	return words
}

func spotifyTrainingText(words []spotifyWord) string {
	parts := make([]string, 0, len(words))
	for _, word := range words {
		parts = append(parts, word.Original)
	}
	return strings.Join(parts, " ")
}

func spotifySplitTrainingTrack(words []spotifyWord) ([]spotifyWord, []spotifyWord, bool) {
	split := -1
	for index, word := range words {
		if word.Folded == "de" && index > 0 && index+1 < len(words) {
			split = index
		}
	}
	if split >= 0 {
		return words[:split], words[split+1:], true
	}
	return nil, nil, false
}

func truncateRunes(value string, max int) string {
	if max < 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) > max {
		return string(runes[:max])
	}
	return value
}
