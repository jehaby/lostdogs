package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type wantCase struct {
	Type             PostType
	Animal           AnimalType
	BreedContains    string
	Phones           []string
	ContactNamesHas  []string
	LocationContains []string
	WhenContains     []string
	AgeContains      string
	ExtrasSterilized bool
	ExtrasVaccinated bool
	ExtrasLitterOK   bool
}

func TestParse(t *testing.T) {
	cases := []struct {
		name string
		text string
		want wantCase
	}{
		{
			name: "Lost cat with address and phone",
			text: "Пропала кошка! Срочно нужна помощь. Пушкинская улица, 283, Ижевск. Телефон: 79120281683. Имя хозяина: Юрий. Кошка Мася",
			want: wantCase{
				Type:             TypeLost,
				Animal:           AnimalCat,
				Phones:           []string{"+79120281683"},
				LocationContains: []string{"Пушкинская улица", "283"},
				ContactNamesHas:  []string{"Юрий"},
			},
		},
		{
			name: "Sighting dog no phone",
			text: "Бегает на Закирова и Первомайской кобелек, похож на Йорка, поймать не смогли",
			want: wantCase{
				Type:             TypeSighting,
				Animal:           AnimalDog,
				BreedContains:    "йорк",
				LocationContains: []string{"Закирова", "Первомайск"},
			},
		},
		{
			name: "Found cat with phone and name",
			text: "Найден кот. Район Заречное шоссе 49. 89127500184 Александр",
			want: wantCase{
				Type:             TypeFound,
				Animal:           AnimalCat,
				Phones:           []string{"+79127500184"},
				ContactNamesHas:  []string{"Александр"},
				LocationContains: []string{"Заречное шоссе", "49"},
			},
		},
		{
			name: "Adoption sterilized vaccinated",
			text: "Красивая кошечка, стерилизована, обработана. Лоток на отлично. 8-912-762-92-39 Ольга",
			want: wantCase{
				Type:             TypeAdoption,
				Animal:           AnimalCat,
				ExtrasSterilized: true,
				ExtrasLitterOK:   true,
				Phones:           []string{"+79127629239"},
				ContactNamesHas:  []string{"Ольга"},
			},
		},
		{
			name: "Fundraising",
			text: "Помочь оплатить передержку. Сумма к сбору. 📞 8912 4586329 Анна",
			want: wantCase{
				Type:            TypeFundraising,
				Phones:          []string{"+79124586329"},
				ContactNamesHas: []string{"Анна"},
			},
		},
		{
			name: "Lost with date time",
			text: "Потерялась кошка. Воткинское шоссе 39 26.08.2025 примерно в 22:00. 89120216801",
			want: wantCase{
				Type:             TypeLost,
				WhenContains:     []string{"26.08.2025"},
				LocationContains: []string{"Воткинское шоссе", "39"},
				Phones:           []string{"+79120216801"},
			},
		},
		{
			name: "Adoption kitten 2 months",
			text: "Малышу около 2х месяцев, лотком пользуется, пишите Юлии 89501684430",
			want: wantCase{
				Type:        TypeAdoption,
				AgeContains: "2",
				Phones:      []string{"+79501684430"},
			},
		},
		{
			name: "Link only",
			text: "https://vk.com/wall107929440_36",
			want: wantCase{Type: TypeLink},
		},
		{
			name: "Empty",
			text: "",
			want: wantCase{Type: TypeEmpty},
		},
		{
			name: "Lost multiple areas with name",
			text: "Мы потеряли кота. Районы Люкшудья и Шабердино. 8 922 405 26 12 - Татьяна",
			want: wantCase{
				Type:             TypeLost,
				Phones:           []string{"+79224052612"},
				ContactNamesHas:  []string{"Татьяна"},
				LocationContains: []string{"Люкшудья", "Шабердино"},
			},
		},
		{
			name: "Adoption extras",
			text: "стерилизована, вакцинирована. Лоток на отлично",
			want: wantCase{
				Type:             TypeAdoption,
				ExtrasSterilized: true,
				ExtrasVaccinated: true,
				ExtrasLitterOK:   true,
			},
		},
		{
			name: "Adoption duplicate content style",
			text: "Кошечка ищет дом, ласковая, 8 906 816 84 18",
			want: wantCase{
				Type:   TypeAdoption,
				Animal: AnimalCat,
				Phones: []string{"+79068168418"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Parse(0, tc.text)

			require.Equal(t, tc.want.Type, got.Type, "Type")

			if tc.want.Animal != "" {
				assert.Equal(t, tc.want.Animal, got.Animal, "Animal")
			}
			if tc.want.BreedContains != "" {
				assert.Contains(t, strings.ToLower(got.Breed), tc.want.BreedContains, "BreedContains")
			}
			if tc.want.AgeContains != "" {
				assert.Contains(t, got.Age, tc.want.AgeContains, "AgeContains")
			}
			if tc.want.Phones != nil {
				assert.Equal(t, tc.want.Phones, got.Phones, "Phones")
			}
			if len(tc.want.LocationContains) > 0 {
				assert.Truef(t, containsAll(got.Location, tc.want.LocationContains), "Location: %q missing %v", got.Location, tc.want.LocationContains)
			}
			if len(tc.want.WhenContains) > 0 {
				assert.Truef(t, containsAll(got.When, tc.want.WhenContains), "When: %q missing %v", got.When, tc.want.WhenContains)
			}
			if len(tc.want.ContactNamesHas) > 0 {
				assert.Truef(t, sliceHasAll(got.ContactNames, tc.want.ContactNamesHas), "ContactNames: %v missing %v", got.ContactNames, tc.want.ContactNamesHas)
			}
			if tc.want.ExtrasSterilized {
				assert.True(t, got.Extras.Sterilized, "Extras.Sterilized")
			}
			if tc.want.ExtrasVaccinated {
				assert.True(t, got.Extras.Vaccinated, "Extras.Vaccinated")
			}
			if tc.want.ExtrasLitterOK {
				assert.True(t, got.Extras.LitterOK, "Extras.LitterOK")
			}
		})
	}
}

// --- helpers ---

func containsAll(hay string, needles []string) bool {
	for _, n := range needles {
		if !strings.Contains(hay, n) {
			return false
		}
	}
	return true
}

func sliceHasAll(hay []string, needles []string) bool {
	set := map[string]bool{}
	for _, v := range hay {
		set[v] = true
	}
	for _, n := range needles {
		if !set[n] {
			return false
		}
	}
	return true
}

func TestExtractPhones(t *testing.T) {
	cases := []struct {
		name string
		text string
		want []string
	}{
		{name: "+7 spaced/dashed", text: "Связь: +7 (912) 028-16-83", want: []string{"+79120281683"}},
		{name: "8 with dashes", text: "Тел: 8-912-762-92-39", want: []string{"+79127629239"}},
		{name: "8 with spaces", text: "📞 8912 4586329 Анна", want: []string{"+79124586329"}},
		{name: "bare 11 starting 7", text: "Телефон: 79120216801", want: []string{"+79120216801"}},
		{name: "mobile 10 starting 9", text: "Звоните 922 405 26 12", want: []string{"+79224052612"}},
		{name: "normalize from 8950...", text: "пишите Юлии 89501684430", want: []string{"+79501684430"}},
		{name: "multiple + order + dedupe", text: "Тел: 8 (912) 028-16-83, +7 912 028 16 83, 922 405 26 12", want: []string{"+79120281683", "+79224052612"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractPhones(tc.text)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestDetectAnimal(t *testing.T) {
	cases := []struct {
		name string
		text string
		want AnimalType
	}{
		{name: "Cat: кошка", text: "Пропала кошка во дворе", want: AnimalCat},
		{name: "Cat: кот", text: "Найден кот у подъезда", want: AnimalCat},
		{name: "Cat: котёнок", text: "Потерялся котёнок, рыжий", want: AnimalCat},
		{name: "Cat: кошечка", text: "Кошечка ищет дом", want: AnimalCat},
		{name: "Cat: бенгальская", text: "Бенгальская красавица ждёт хозяйку", want: AnimalCat},

		{name: "Dog: собака", text: "Нашли собаку возле школы", want: AnimalDog},
		{name: "Dog: пёс", text: "Пёс бегает по району", want: AnimalDog},
		{name: "Dog: песик", text: "Милый песик ищет дом", want: AnimalDog},
		{name: "Dog: кобелек", text: "Кобелек йорк", want: AnimalDog},
		{name: "Dog: щенок", text: "Щенок найден ночью", want: AnimalDog},

		{name: "Unknown: neutral text", text: "Привет всем! Отличный день.", want: AnimalUnknown},
		{name: "Unknown: avoids котейка", text: "Слово котейка не должно сработать", want: AnimalUnknown},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := detectAnimal(tc.text)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestDetectType(t *testing.T) {
	cases := []struct {
		name string
		text string
		want PostType
	}{
		{name: "Lost: пропала", text: "Пропала кошка, Ижевск", want: TypeLost},
		{name: "Lost: потерялся", text: "Потерялся пёсик дворняга", want: TypeLost},
		{name: "Lost: убежал", text: "Убежал котик вчера", want: TypeLost},

		{name: "Found: найден", text: "Найден кот во дворе", want: TypeFound},
		{name: "Found: нашли", text: "Нашли собаку у магазина", want: TypeFound},
		{name: "Found: подобрали", text: "Подобрали щенка у подъезда", want: TypeFound},

		{name: "Sighting: бегает", text: "Бегает кобелек во дворе", want: TypeSighting},
		{name: "Sighting: замечена", text: "Замечена собака у школы", want: TypeSighting},
		{name: "Sighting: видели", text: "Видели кота на лестнице", want: TypeSighting},

		{name: "Adoption: ищет дом", text: "Кошечка ищет дом, ласковая", want: TypeAdoption},
		{name: "Adoption: добрые руки", text: "Отдаем котенка в добрые руки", want: TypeAdoption},
		{name: "Adoption: care markers only", text: "Стерилизована, привита, лоток на отлично", want: TypeAdoption},

		{name: "Fundraising: сбор", text: "Сбор на оплату передержки", want: TypeFundraising},
		{name: "Fundraising: перевод/карта", text: "Нужен перевод на карту Сбер", want: TypeFundraising},

		{name: "Tie both words → Found (найден...)", text: "Найден кот, потом потерялся", want: TypeFound},
		{name: "Tie both words → Lost (без 'найден')", text: "Нашли собаку, но пропала позже", want: TypeLost},

		{name: "Unknown", text: "Привет всем! Отличный день.", want: TypeUnknown},

		{name: "Adoption vs Fund → Adoption priority", text: "Ищет дом, возможен сбор на корм", want: TypeAdoption},
		{name: "Adoption via care markers: вакцинирована/чипирована", text: "Кошка вакцинирована и чипирована", want: TypeAdoption},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := detectType(tc.text)
			require.Equal(t, tc.want, got)
		})
	}
}

func sliceHasAny(hay []string, needles []string) bool {
	set := map[string]bool{}
	for _, v := range hay {
		set[v] = true
	}
	for _, n := range needles {
		if set[n] {
			return true
		}
	}
	return false
}

// Using testify for equality checks in tests; helpers above cover contains cases.
