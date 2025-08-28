this is codex-friendly **implementation plan** that yields **pure (side-effect-free)** code in two files: `domain.go` and `domain_test.go`. It sticks to **regexp + simple heuristics** only.

---

# domain.go — Spec

## Package

`package vkdom`

## Data types

```go
// Canonical post result (pure data, no side effects)
type Post struct {
    ID            int            // not parsed from text; tests can set it
    Raw           string         // original text
    Type          PostType       // classified type
    Animal        AnimalType     // cat/dog/other/unknown
    Breed         string         // extracted breed or ""
    Sex           SexType        // m/f/unknown
    Age           string         // free text (e.g., "2,5 мес", "6-7 месяцев")
    Name          string         // pet name if confidently extracted (e.g., "Мася")
    Location      string         // simple heuristic span (street+number/city/area)
    When          string         // free text date/time if confidently found (e.g., "26.08.2025 22:00")
    Phones        []string       // normalized to +7XXXXXXXXXX
    ContactNames  []string       // human names near phones or vk mentions
    VKAccounts    []string       // vk.com links or [id...|Name] mentions (original form)
    Extras        Extras         // boolean flags
    StatusDetails string         // short trimmed summary of key descriptors
}

// Controlled enums (stringer not necessary)
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
```

## Public API

```go
// Parse returns a fully-populated Post from raw text.
// Pure function: no I/O, no globals, deterministic.
func Parse(id int, raw string) Post
```

## Helpers (unexported; pure)

Implement these as small, testable pieces used by `Parse`:

```go
func normalizeSpace(s string) string
func detectType(s string) PostType
func extractPhones(s string) []string          // normalized to +7...
func extractVKAccounts(s string) []string
func extractContactNamesAroundPhones(s string, phones []string) []string
func detectAnimal(s string) AnimalType
func extractBreed(s string) string
func detectSex(s string) SexType
func extractAge(s string) string
func extractWhen(s string) string
func extractLocationHeuristic(s string) string
func extractStatusDetails(s string) string
func extractPetName(s string) string
```

## Regex/heuristics (exact patterns to use)

* **Lost**:

  * `\b(пропал[аи]?|потерял[асься]?|убежал[аи]?|сбежал[аи]?)\b`

* **Found**:

  * `\b(найден[аоы]?|нашл[аи]|подобрал[аи])\b`

* **Sighting**:

  * `\b(замечен[ао]?|видел[аи]?|бегает|появил[асься])\b`

* **Adoption**:

  * `\b(ищет\s+дом|в\s+добрые\s+руки|отда[ёмм]|пристраив[ае])\b`
  * plus “care” markers: `стерилиз|кастрир|вакц|привит|чипир|лоток`

* **Fundraising**:

  * `\b(сбор|оплатить|перевод|передержк|карта)\b`

* **Link/Empty**:

  * if trimmed text is empty → `TypeEmpty`
  * if contains only URLs/ids and ≤20 non-URL chars → `TypeLink`

* **Tie-break rule**: If both lost/found matched, prefer the *more specific* sentence fragment:

  * If any `найден.*(кот|собак|пёс|пес|кобел|щен|живот)` → Found
  * Else if any lost words present → Lost

* **Phones** (Russian, normalize):

  * Raw match:

    ```
    (?:(?:\+7|8)\s*\(?\d{3}\)?[\s-]?\d{3}[\s-]?\d{2}[\s-]?\d{2}|\b9\d{2}[\s-]?\d{3}[\s-]?\d{2}[\s-]?\d{2}\b)
    ```
  * Normalization:

    * keep only digits
    * if starts with "8" and length 11 → replace first with "+7"
    * if starts with "7" and length 11 → prefix "+" → "+7..."
    * if starts with "9" and length 10/11 → prefix "+7"
    * final expected `+7XXXXXXXXXX` (11 digits total after +7)
    * dedupe while preserving first-seen order

* **Animal**:

  * Cat if matches: `кошк|кот(?!ейка)|кот[её]н|кис[ао]ньк|бенгальск`
  * Dog if matches: `собак|пс|п[её]с\b|п[её]сик|кобел|щен`
  * Else unknown

* **Breed**: grab known fragments near animal words (case-insensitive):

  * `бенгальск[аяийое]`, `йорк(шир(ск(ий|ая))?)?`, `лабрадор`, `овчарк`, `хаск`, `такс`, `спаниел`, `метис`
  * Return the matched token (title-cased)

* **Sex**:

  * Male: `\bкобел(ё|е)к|мальчик\b`
  * Female: `\bдевочк|сука\b`

* **Age** (loose):

  * Short capture: `(\d+[,.]?\d*|\d+\s*-\s*\d+)\s*(мес|месяц|месяцев|год|года|лет)`
  * Return the first concise match (normalize commas to dots)

* **When** (date/time):

  * Date: `\b\d{1,2}[.]\d{1,2}[.]\d{2,4}\b`
  * Time near date: `(?:в\s*)?\b\d{1,2}[:.]\d{2}\b`
  * If both present near each other, join `"DD.MM.YYYY HH:MM"`, else just date

* **Location heuristic**:

  * Find spans with tokens like:
    `улиц|шосс|просп|пер(е)?ул|площад|бульвар|район|снт|город|деревн|пос(е)?лок|Ижевск|Закирова|Первомайск|Люкшудья|Шабердино|Воткинск`
  * Choose the shortest comma-delimited segment that contains one trigger and (optionally) a number; trim.

* **VK accounts**:

  * `vk\.com/\S+`
  * `\[(id\d+)\|([^\]]+)\]` (keep the whole `[id...|Name]`)

* **Contact names**:

  * Within ±25 chars of a phone, capture a single capitalized word in Cyrillic (`\b[А-ЯЁ][а-яё]{2,}\b`), exclude common nouns.
  * Add also from VK mention `[id..|Имя Фамилия]` → split and take first word.
  * Dedupe.

* **StatusDetails**:

  * Heuristic: collect up to \~140 chars of adjectives/traits from keywords:
    `рыж|белоснежн|пуглив|ласков|игрив|домашн|без ошейн|кастрир|стерилиз|вакцин|чипир|лоток`
  * Join unique snippets with `, `

* **Pet name** (weak heuristic):

  * Pattern: `кличка\s+([A-ZА-ЯЁ][\p{L}\-]{2,})` OR `кошк[аи]\s+([A-ZА-ЯЁ][\p{L}\-]{2,})` within 20 chars of “кличка” or “зовут”.
  * If multiple, take the first.

## Parse flow

1. `s := normalizeSpace(raw)`
2. If empty → `TypeEmpty`
3. Detect `Type` (apply tie-break).
4. Extract phones → normalize & dedupe.
5. Extract VK accounts.
6. Detect Animal, Breed, Sex, Age.
7. Extract When, Location.
8. Extract Contact Names (using phones + VK mentions).
9. Build `StatusDetails` and `Name`.
10. Return `Post`.

---

# domain\_test.go — Tests Plan

`package vkdom`

Use **table-driven tests**. No external deps. Each case provides `id`, `text`, and expected fields minimally necessary to validate the logic. Keep tests deterministic and small.

## Sample cases (from your real messages)

1. **Lost cat with full address + phone** (id 11 or 20)

```go
text := "Пропала кошка! ... Пушкинская улица, 283, Ижевск ... Телефон: 79120281683 ... Имя хозяина: Юрий ... кошка Мася"
Expect:
Type=TypeLost
Animal=AnimalCat
Phones=["+79120281683"]
Location contains "Пушкинская улица, 283"
ContactNames includes "Юрий"
Name == "Мася" (optional if “кличка” absent; acceptable to not match — keep test tolerant)
```

2. **Sighting dog, no phone** (id 22)

```go
"Бегает на Закирова и Первомайской кобелек ... похож на Йорка ... поймать не смогли"
Type=TypeSighting
Animal=AnimalDog
Breed contains "йорк"
Location contains "Закирова" and/or "Первомайской"
len(Phones)=0
```

3. **Found cat with phone & name** (id 46)

```go
"Найден кот. Район Заречное шоссе 49 ... 89127500184 Александр"
Type=TypeFound
Animal=AnimalCat
Phones=["+79127500184"]
ContactNames contains "Александр"
Location contains "Заречное шоссе 49"
```

4. **Adoption, sterilized, vaccinated** (id 18)

```go
"красивая кошечка ... стерилизована, обработана ... Лоток на отлично ... 8-912-762-92-39 Ольга"
Type=TypeAdoption
Animal=AnimalCat
Extras.Sterilized=true
Extras.LitterOK=true
Phones has "+79127629239"
ContactNames contains "Ольга"
```

5. **Fundraising** (id 14)

```go
"помочь оплатить передержку ... Сумма к сбору ... 📞 8912 4586329 Анна"
Type=TypeFundraising
Phones ["+79124586329"]
ContactNames contains "Анна"
```

6. **Lost with explicit date/time** (id 36)

```go
"Потерялась кошка ... Воткинское шоссе 39 26.08.2025 примерно в 22ч ... 89120216801"
Type=TypeLost
When contains "26.08.2025"   // time normalization tolerant ("22" or "22:00")
Location contains "Воткинское шоссе 39"
Phones ["+79120216801"]
```

7. **Adoption (kitten 2 months)** (id 27 or 40/41/42)

```go
"Малышу около 2х месяцев ... лотком ... пишите Юлии 89501684430"
Type=TypeAdoption
Age contains "2"
Phones ["+789501684430" → "+789..." is invalid; expect "+7" normalization from "8950..." → "+789501684430" WRONG]
Correct expectation: "+7 950 168-44-30" → "+79501684430"
```

8. **Link-only** (id 21 or 45 or 49)

```go
"https://vk.com/wall107929440_36"
Type=TypeLink
```

9. **Empty** (id 12 or 24 or 30)

```go
""
Type=TypeEmpty
```

10. **Lost (long story across areas)** (id 32)

```go
"... мы потеряли кота ... Люкшудья и Шабердино ... 89224052612- Татьяна"
Type=TypeLost
Phones ["+789224052612" → normalize to "+789..."?] // Correct: Russian: 8 922 405 26 12 → "+79224052612"
ContactNames includes "Татьяна"
Location contains "Люкшудья" or "Шабердино"
```

11. **Adoption extras** (id 29)

```go
"стерилизована, вакцинирована. Лоток на отлично"
Type=TypeAdoption
Extras.Sterilized=true
Extras.Vaccinated=true
Extras.LitterOK=true
```

12. **Duplicate content check (id 19 vs 44)**
    Both are similar adoption/rescue ads; not deduped here (no persistence). Just ensure consistent classification:

```go
Type=TypeAdoption
Animal=AnimalCat
Phones contain "+79068168418"
```

> Note: in tests 7 and 10 I highlighted the typical *normalization gotchas*. Put the **correct** expectations in code; comments can mention the original messy forms. (See expected arrays below.)

## Test structure

* `TestParse_Table` with an array of `{name, id, text, want}`.
* `want` can be a minimal struct or per-field checks to keep tests robust:

  * Always check `Type`
  * Check `Phones` (exact normalized slice)
  * For strings like `Location`, assert `strings.Contains(got.Location, "...")`
  * For booleans, direct equality
  * For `Animal`, `Sex`, `Extras`, direct equality

### Example expected values (ready to paste)

Use exactly these in the test table:

```go
// Case 1 (id=11)
want.Type = TypeLost
want.Animal = AnimalCat
want.Phones = []string{"+79120281683"}
want.LocationContains = []string{"Пушкинская улица", "283"}
want.ContactNamesHas = []string{"Юрий"}

// Case 2 (id=22)
want.Type = TypeSighting
want.Animal = AnimalDog
want.BreedContains = "йорк"
want.LocationContains = []string{"Закирова", "Первомайск"}
want.Phones = nil

// Case 3 (id=46)
want.Type = TypeFound
want.Animal = AnimalCat
want.Phones = []string{"+79127500184"}
want.ContactNamesHas = []string{"Александр"}
want.LocationContains = []string{"Заречное шоссе", "49"}

// Case 4 (id=18)
want.Type = TypeAdoption
want.Animal = AnimalCat
want.ExtrasSterilized = true
want.ExtrasLitterOK = true
want.Phones = []string{"+79127629239"}
want.ContactNamesHas = []string{"Ольга"}

// Case 5 (id=14)
want.Type = TypeFundraising
want.Phones = []string{"+79124586329"}
want.ContactNamesHas = []string{"Анна"}

// Case 6 (id=36)
want.Type = TypeLost
want.WhenContains = []string{"26.08.2025"}
want.LocationContains = []string{"Воткинское шоссе", "39"}
want.Phones = []string{"+79120216801"}

// Case 7 (id=27)
want.Type = TypeAdoption
want.AgeContains = "2"
want.Phones = []string{"+79501684430"} // from 8950...

// Case 8 (id=21)
want.Type = TypeLink

// Case 9 (id=12)
want.Type = TypeEmpty

// Case 10 (id=32)
want.Type = TypeLost
want.Phones = []string{"+79224052612"}
want.ContactNamesHas = []string{"Татьяна"}
want.LocationContains = []string{"Люкшудья", "Шабердино"}

// Case 11 (id=29)
want.Type = TypeAdoption
want.ExtrasSterilized = true
want.ExtrasVaccinated = true
want.ExtrasLitterOK = true

// Case 12 (id=44)
want.Type = TypeAdoption
want.Animal = AnimalCat
want.Phones = []string{"+79068168418"} // from "8 906 816 84 18"
```

## Test utilities

Inside `domain_test.go`, add tiny helpers:

```go
func containsAll(hay string, needles []string) bool
func sliceHasAll(hay []string, needles []string) bool
func sliceHasAny(hay []string, needles []string) bool
func mustEqualSlices(t *testing.T, got, want []string)
```

Assert with `t.Fatalf/t.Errorf` only.

---

# Notes/constraints for Codex

* No `init()`, no I/O, no time, no randomness, no external packages.
* Use `regexp` and `strings`, `unicode`, `sort`.
* Keep all regex compiled once at package level:

  ```go
  var reLost = regexp.MustCompile(`(?i)\b(пропал[аи]?|потерял[асься]?|убежал[аи]?|сбежал[аи]?)\b`)
  // ...etc
  ```
* All helpers must be **pure**: input string → output value, with no global mutations.
* Dedupe functions should preserve input order (use small map\[string]struct{} guard).
* Keep `extractLocationHeuristic` simple: split on `[,.;\n]`, filter segments with trigger tokens, choose shortest.

---

If you want, I can turn this plan into actual `domain.go` + `domain_test.go` scaffolding next — but this spec is already “codex-ready”: drop it in and let the tool implement functions and tests to match.
