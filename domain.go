package main

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// Canonical post result (pure data, no side effects)
type Post struct {
	ID            int
	Raw           string
	Type          PostType
	Animal        AnimalType
	Breed         string
	Sex           SexType
	Age           string
	Name          string
	Location      string
	When          string
	Phones        []string
	ContactNames  []string
	VKAccounts    []string
	Extras        Extras
	StatusDetails string
}

// Controlled enums
type PostType string

const (
	TypeUnknown     PostType = "unknown"
	TypeLost        PostType = "lost"
	TypeFound       PostType = "found"
	TypeSighting    PostType = "sighting"
	TypeAdoption    PostType = "adoption"
	TypeFundraising PostType = "fundraising"
	TypeNews        PostType = "news"
	TypeLink        PostType = "link"
	TypeEmpty       PostType = "empty"
)

type AnimalType string

const (
	AnimalUnknown AnimalType = "unknown"
	AnimalCat     AnimalType = "cat"
	AnimalDog     AnimalType = "dog"
	AnimalOther   AnimalType = "other"
)

type SexType string

const (
	SexUnknown SexType = "unknown"
	SexM       SexType = "m"
	SexF       SexType = "f"
)

type Extras struct {
	Sterilized bool
	Vaccinated bool
	Chipped    bool
	LitterOK   bool
}

// Compiled regexes (case-insensitive where needed)
var (
	reSpace       = regexp.MustCompile(`\s+`)
	reLost        = regexp.MustCompile(`(?i)(пропал[аи]?|потерял[асься]?|убежал[аи]?|сбежал[аи]?)`)
	reFound       = regexp.MustCompile(`(?i)(найден[аоы]?|нашл[аи]|подобрал[аи])`)
	reSighting    = regexp.MustCompile(`(?i)(замечен[ао]?|видел[аи]?|бегает|появил[асься])`)
	reAdoption    = regexp.MustCompile(`(?i)(ищет\s+дом|в\s+добрые\s+руки|отда[ёмм]|пристраив[ае])`)
	reCareMarkers = regexp.MustCompile(`(?i)(стерилиз|кастрир|вакц|привит|чипир|лоток|лотк)`) // for adoption leaning
	// Use Unicode-safe boundaries: ASCII \b doesn't handle Cyrillic correctly.
	reFundraising  = regexp.MustCompile(`(?i)(^|[^\p{L}\d])(сбор|оплатить|перевод|передержк|карта)([^\p{L}\d]|$)`)
	reTieFoundSpec = regexp.MustCompile(`(?i)найден\S*.*(кот|собак|п[её]с|кобел|щен|живот)`) // specific found pattern

	rePhone = regexp.MustCompile(`(?:(?:\+7|8)\s*\(?\d{3}\)?[\s-]?\d{3}[\s-]?\d{2}[\s-]?\d{2}|\b9\d{2}[\s-]?\d{3}[\s-]?\d{2}[\s-]?\d{2}\b|\b7\d{10}\b)`)

	reVKURL     = regexp.MustCompile(`(?i)vk\.com/\S+`)
	reVKBracket = regexp.MustCompile(`\[(id\d+)\|([^\]]+)\]`)

	// Note: Go RE2 doesn't support negative lookahead; keep it simple.
	// Avoid ASCII word-boundaries; use explicit fragments and common punctuation/space/end checks.
	reCat  = regexp.MustCompile(`(?i)кошечк|кошк|кот(?:\s|[.,!?:;]|$)|кот[её]н|кис[ао]ньк|бенгальск`)
	reDog  = regexp.MustCompile(`(?i)собак|пс|п[её]с(?:\s|$)|п[её]сик|кобел|щен`)
	reMale = regexp.MustCompile(`(?i)кобел(ё|е)к|мальчик`)
	reFem  = regexp.MustCompile(`(?i)девочк|сука`)

	reBreed = regexp.MustCompile(`(?i)бенгальск[аяийое]|йорк(?:шир(?:ск(?:ий|ая))?)?|лабрадор|овчарк|хаск|такс|спаниел|метис`)

	// Allow compact forms like "2х месяцев" (optional x/х after number).
	reAge  = regexp.MustCompile(`(?i)(\d+[,.]?\d*|\d+\s*-\s*\d+)\s*[xх]?\s*(мес|месяц|месяцев|год|года|лет)`) // capture first concise
	reDate = regexp.MustCompile(`\b\d{1,2}[.]\d{1,2}[.]\d{2,4}\b`)
	reTime = regexp.MustCompile(`(?i)(?:в\s*)?\b\d{1,2}[:.]\d{2}\b`)

	reLocTrigger = regexp.MustCompile(`(?i)улиц|шосс|просп|пер(?:е)?ул|площад|бульвар|район|снт|город|деревн|пос(?:е)?лок|Ижевск|Закирова|Первомайск|Люкшудья|Шабердино|Воткинск`)

	reCapCyrWord = regexp.MustCompile(`\b[А-ЯЁ][а-яё]{2,}\b`)

	reStatus = regexp.MustCompile(`(?i)рыж|белоснежн|пуглив|ласков|игрив|домашн|без\s*ошейн|кастрир|стерилиз|вакцин|чипир|лоток`)

	reName1 = regexp.MustCompile(`(?i)(?:кличка|зовут)[^\n]{0,20}\s+([A-Za-zА-Яа-яЁё][\p{L}-]{2,})`)
	reName2 = regexp.MustCompile(`(?i)кошк[аи][^\n]{0,20}\s+([A-Za-zА-Яа-яЁё][\p{L}-]{2,})`)
)

// Parse returns a fully-populated Post from raw text. Pure and deterministic.
func Parse(id int, raw string) Post {
	s := normalizeSpace(raw)
	p := Post{ID: id, Raw: raw}
	if strings.TrimSpace(s) == "" {
		p.Type = TypeEmpty
		return p
	}

	// Accounts and link-only detection helpers
	vkAcc := extractVKAccounts(s)
	if isLinkOnly(s) {
		p.Type = TypeLink
		p.VKAccounts = vkAcc
		return p
	}

	p.Type = detectType(s)
	p.Phones = extractPhones(s)
	p.VKAccounts = vkAcc
	p.Animal = detectAnimal(s)
	p.Breed = extractBreed(s)
	p.Sex = detectSex(s)
	p.Age = extractAge(s)
	p.When = extractWhen(s)
	p.Location = extractLocationHeuristic(s)
	names := extractContactNamesAroundPhones(s, p.Phones)
	// also add first names from VK mentions
	names = append(names, extractNamesFromMentions(s)...)
	p.ContactNames = dedupeKeepOrder(names)
	p.Extras = Extras{
		Sterilized: reCareMarkers.MatchString(s) && regexp.MustCompile(`(?i)стерилиз|кастрир`).MatchString(s),
		Vaccinated: regexp.MustCompile(`(?i)вакцин|привит`).MatchString(s),
		Chipped:    regexp.MustCompile(`(?i)чипир`).MatchString(s),
		LitterOK:   regexp.MustCompile(`(?i)лоток`).MatchString(s),
	}
	p.StatusDetails = extractStatusDetails(s)
	p.Name = extractPetName(s)
	return p
}

func normalizeSpace(s string) string {
	s = strings.ReplaceAll(s, "\u00A0", " ")
	return strings.TrimSpace(reSpace.ReplaceAllString(s, " "))
}

func detectType(s string) PostType {
	lost := reLost.MatchString(s)
	found := reFound.MatchString(s)
	sight := reSighting.MatchString(s)
	adopt := reAdoption.MatchString(s) || reCareMarkers.MatchString(s)
	fund := reFundraising.MatchString(s)

	if found && lost {
		if reTieFoundSpec.MatchString(s) {
			return TypeFound
		}
		return TypeLost
	}
	if found {
		return TypeFound
	}
	if lost {
		return TypeLost
	}
	if sight {
		return TypeSighting
	}
	if adopt {
		return TypeAdoption
	}
	if fund {
		return TypeFundraising
	}
	return TypeUnknown
}

func extractPhones(s string) []string {
	m := rePhone.FindAllString(s, -1)
	seen := make(map[string]bool)
	out := make([]string, 0, len(m))
	for _, raw := range m {
		digits := onlyDigits(raw)
		if len(digits) == 11 && digits[0] == '8' {
			digits = "7" + digits[1:]
		}
		if len(digits) == 11 && digits[0] == '7' {
			norm := "+" + digits
			if !seen[norm] {
				seen[norm] = true
				out = append(out, norm)
			}
			continue
		}
		if len(digits) == 10 && digits[0] == '9' {
			norm := "+7" + digits
			if !seen[norm] {
				seen[norm] = true
				out = append(out, norm)
			}
		}
	}
	return out
}

func extractVKAccounts(s string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, v := range reVKBracket.FindAllString(s, -1) {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	for _, v := range reVKURL.FindAllString(s, -1) {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func extractContactNamesAroundPhones(s string, phones []string) []string {
	// Get positions of phone matches in original string
	idxs := rePhone.FindAllStringIndex(s, -1)
	names := []string{}
	for _, rg := range idxs {
		start := rg[0] - 25
		if start < 0 {
			start = 0
		}
		end := rg[1] + 25
		if end > len(s) {
			end = len(s)
		}
		window := s[start:end]
		for _, m := range reCapCyrWord.FindAllString(window, -1) {
			if isCommonNoun(m) {
				continue
			}
			names = append(names, m)
		}
	}
	return dedupeKeepOrder(names)
}

func detectAnimal(s string) AnimalType {
	if reCat.MatchString(s) {
		return AnimalCat
	}
	if reDog.MatchString(s) {
		return AnimalDog
	}
	return AnimalUnknown
}

func extractBreed(s string) string {
	b := reBreed.FindString(s)
	if b == "" {
		return ""
	}
	return titleCase(b)
}

func detectSex(s string) SexType {
	if reMale.MatchString(s) {
		return SexM
	}
	if reFem.MatchString(s) {
		return SexF
	}
	return SexUnknown
}

func extractAge(s string) string {
	m := reAge.FindStringSubmatch(s)
	if len(m) == 0 {
		return ""
	}
	val := strings.ReplaceAll(m[1], ",", ".")
	unit := m[2]
	return normalizeSpace(val + " " + unit)
}

func extractWhen(s string) string {
	loc := reDate.FindStringIndex(s)
	if loc == nil {
		return ""
	}
	date := s[loc[0]:loc[1]]
	// look ahead for time near date
	// search within next 30 chars
	end := loc[1] + 30
	if end > len(s) {
		end = len(s)
	}
	after := s[loc[1]:end]
	tm := reTime.FindString(after)
	if tm != "" {
		tm = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(tm), "в"))
		tm = strings.ReplaceAll(tm, ".", ":")
		return strings.TrimSpace(date + " " + tm)
	}
	return date
}

func extractLocationHeuristic(s string) string {
	// Split by commas and pick shortest segment with a trigger.
	// If the next comma-separated segment is a numeric-like house number,
	// append it to the candidate (e.g., "Пушкинская улица, 283" → "Пушкинская улица 283").
	parts := strings.Split(s, ",")
	best := ""
	for i := 0; i < len(parts); i++ {
		t := strings.TrimSpace(parts[i])
		if t == "" {
			continue
		}
		if reLocTrigger.MatchString(t) {
			cand := t
			if i+1 < len(parts) {
				nxt := strings.TrimSpace(parts[i+1])
				if nxt != "" && isNumericLike(nxt) {
					cand = strings.TrimSpace(cand + " " + nxt)
				}
			}
			if best == "" || len([]rune(cand)) < len([]rune(best)) {
				best = cand
			}
		}
	}
	return best
}

func isNumericLike(s string) bool {
	// true if runes are digits, spaces or dashes only and at least one digit present
	hasDigit := false
	for _, r := range s {
		if r >= '0' && r <= '9' {
			hasDigit = true
			continue
		}
		if r == ' ' || r == '-' {
			continue
		}
		return false
	}
	return hasDigit
}

func extractStatusDetails(s string) string {
	ms := reStatus.FindAllString(s, -1)
	if len(ms) == 0 {
		return ""
	}
	uniq := dedupeKeepOrder(ms)
	out := strings.Join(uniq, ", ")
	if len([]rune(out)) > 140 {
		r := []rune(out)
		out = string(r[:140])
	}
	return out
}

func extractPetName(s string) string {
	if m := reName1.FindStringSubmatch(s); len(m) > 1 {
		return titleCase(m[1])
	}
	if m := reName2.FindStringSubmatch(s); len(m) > 1 {
		return titleCase(m[1])
	}
	return ""
}

// Helpers
func isLinkOnly(s string) bool {
	tmp := reVKURL.ReplaceAllString(s, "")
	tmp = reVKBracket.ReplaceAllString(tmp, "")
	tmp = strings.TrimSpace(tmp)
	// Count non-space characters
	count := 0
	for _, r := range tmp {
		if !unicode.IsSpace(r) {
			count++
		}
	}
	return count <= 20
}

func extractNamesFromMentions(s string) []string {
	out := []string{}
	for _, m := range reVKBracket.FindAllStringSubmatch(s, -1) {
		if len(m) >= 3 {
			name := strings.TrimSpace(m[2])
			parts := strings.Fields(name)
			if len(parts) > 0 {
				out = append(out, parts[0])
			}
		}
	}
	return dedupeKeepOrder(out)
}

func onlyDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func isCommonNoun(s string) bool {
	low := strings.ToLower(s)
	commons := []string{"Телефон", "Район", "Улица", "Кошка", "Собака", "Проспект", "Бульвар"}
	for _, c := range commons {
		if strings.ToLower(c) == low {
			return true
		}
	}
	return false
}

func titleCase(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

func dedupeKeepOrder(ss []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(ss))
	for _, v := range ss {
		if v == "" {
			continue
		}
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

// Ensure deterministic order where needed
func sortedUnique(ss []string) []string {
	ss = dedupeKeepOrder(ss)
	sort.Strings(ss)
	return ss
}
