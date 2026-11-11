// Package catalog holds the shopping catalogue: the list of products the app
// can compare, with the regexes used to find each one in the chain price files.
package catalog

import "regexp"

type Unit string

const (
	UnitKg   Unit = "kg"
	UnitUnit Unit = "unit"
)

// Item is one comparable product. Include/Exclude are regexes matched against
// the Hebrew product name in the chain files, which are short and abbreviated
// (תפו"א, ג.צהובה, דנונ.פרו).
type Item struct {
	ID         string  `json:"id"`
	He         string  `json:"he"`
	En         string  `json:"en"`
	Category   string  `json:"category"`
	Unit       Unit    `json:"unit"`
	DefaultQty float64 `json:"defaultQty"`
	Include    string  `json:"-"`
	Exclude    string  `json:"-"`
	// PackLabel prefers a standard pack size (e.g. "400" for 400 g cheese).
	PackLabel string `json:"packLabel,omitempty"`

	inc  *regexp.Regexp
	exc  *regexp.Regexp
	pack *regexp.Regexp
}

// Items is the catalogue. unit kg -> price files give a per-kg price that is
// multiplied by the quantity; unit unit -> one pack, preferring PackLabel.
var Items = []Item{
	// ---------- vegetables & fruit (per kg) ----------
	{ID: "cucumber", He: "מלפפון", En: "Cucumber", Category: "ירקות", Unit: UnitKg, DefaultQty: 1,
		Include: `^מלפפון`, Exclude: `במלח|בחומץ|חמוץ|כבוש|קורנישון|בייבי|פרוס|מלפפון/ירקות`},
	{ID: "tomato", He: "עגבניה", En: "Tomato", Category: "ירקות", Unit: UnitKg, DefaultQty: 1,
		Include: `^עגבני`, Exclude: `רסק|מטוגנ|מיובש|שרי|יבש|מיץ|כבוש|ממולא|מרוסק|סלט|אפוי|תמנון`},
	{ID: "potato", He: "תפוח אדמה", En: "Potato", Category: "ירקות", Unit: UnitKg, DefaultQty: 2,
		Include: `תפוח(י)? אדמה|תפו.?א`, Exclude: `מטוגן|צ.?יפס|קפוא|אפוי|פריכ|מחית|חטיף|מקלות|פירה|ממולא`},
	{ID: "onion", He: "בצל", En: "Onion", Category: "ירקות", Unit: UnitKg, DefaultQty: 1,
		Include: `^בצל`, Exclude: `מטוגן|פריכ|יבש|אבקה|כבוש|עוג|בצל ירוק|סלט|מוקפץ`},
	{ID: "carrot", He: "גזר", En: "Carrot", Category: "ירקות", Unit: UnitKg, DefaultQty: 1,
		Include: `^גזר`, Exclude: `מטוגן|קפוא|כבוש|יבש|פרוס|מתוק|אפוי|מקלות|עוג|סלט|מיץ`},
	{ID: "pepper", He: "פלפל", En: "Bell pepper", Category: "ירקות", Unit: UnitKg, DefaultQty: 1,
		Include: `^פלפל`, Exclude: `חריף|שחור|לבן|מטוגן|קלוי|כבוש|יבש|אבקה|בשמן|שוקולד|ממולא|שרי`},
	{ID: "lettuce", He: "חסה", En: "Lettuce", Category: "ירקות", Unit: UnitKg, DefaultQty: 1,
		Include: `^חסה|לקטוס`, Exclude: `כרוב|תרד|אייסברג|סלט`},
	{ID: "cabbage", He: "כרוב", En: "Cabbage", Category: "ירקות", Unit: UnitKg, DefaultQty: 1,
		Include: `^כרוב`, Exclude: `כבוש|סלט|אדום|סיני|ברוקולי|ניצנים|ממולא`},
	{ID: "eggplant", He: "חציל", En: "Eggplant", Category: "ירקות", Unit: UnitKg, DefaultQty: 1,
		Include: `^חציל`, Exclude: `מטוגן|כבוש|בשמן|קלוי|אפוי|סלט|ממולא`},
	{ID: "zucchini", He: "קישוא", En: "Zucchini", Category: "ירקות", Unit: UnitKg, DefaultQty: 1,
		Include: `קישוא|זוקיני`, Exclude: `מטוגן|כבוש|ממולא|אפוי|סלט|פרוס`},
	{ID: "lemon", He: "לימון", En: "Lemon", Category: "פירות", Unit: UnitKg, DefaultQty: 0.5,
		Include: `^לימון`, Exclude: `מיץ|עוג|מלח|כבוש|יבש|קלוף|חמוץ|גזע|משקה|לימונענע|חליט`},
	{ID: "apple", He: "תפוח", En: "Apple", Category: "פירות", Unit: UnitKg, DefaultQty: 1,
		Include: `^תפוח`, Exclude: `אדמה|מטוגן|יבש|אבקה|מיץ|מעובד|אפוי|עוג|דבש|רסק|צ.?יפס|סלט`},
	{ID: "banana", He: "בננה", En: "Banana", Category: "פירות", Unit: UnitKg, DefaultQty: 1,
		Include: `^בננה`, Exclude: `יבש|צ.?יפס|מטוגן|עוג|מיץ|רסק|חליט|שוקולד`},
	{ID: "orange", He: "תפוז", En: "Orange", Category: "פירות", Unit: UnitKg, DefaultQty: 1,
		Include: `^תפוז`, Exclude: `מיץ|משקה|יבש|קלוף|עוג|סוכר|שוקולד`},
	{ID: "avocado", He: "אבוקדו", En: "Avocado", Category: "פירות", Unit: UnitKg, DefaultQty: 1,
		Include: `^אבוקדו`, Exclude: `יבש|מטוגן|עוג|ממרח|שמן|סלט|קפוא`},
	{ID: "grapes", He: "ענבים", En: "Grapes", Category: "פירות", Unit: UnitKg, DefaultQty: 1,
		Include: `^ענבים`, Exclude: `צימוק|מיץ|יבש|יין|עוג|חליט`},
	{ID: "watermelon", He: "אבטיח", En: "Watermelon", Category: "פירות", Unit: UnitKg, DefaultQty: 1,
		Include: `^אבטיח`, Exclude: `גרעינ|מיץ|יבש|עוג|ממתק`},
	{ID: "melon", He: "מלון", En: "Melon", Category: "פירות", Unit: UnitKg, DefaultQty: 1,
		Include: `^מלון`, Exclude: `יבש|מיץ|עוג|בשמן|חליט|מלונית`},
	{ID: "strawberry", He: "תות שדה", En: "Strawberry", Category: "פירות", Unit: UnitKg, DefaultQty: 0.5,
		Include: `^תות`, Exclude: `יבש|ריבה|משקה|עוג|שוקולד|מסטיק|מיץ|ממולא|פרוס|גומי|חליט`},

	// ---------- protein ----------
	{ID: "chicken-breast", He: "חזה עוף", En: "Chicken breast", Category: "בשר ועוף", Unit: UnitKg, DefaultQty: 1.5,
		Include: `חזה עוף|פילה עוף`,
		Exclude: `קפוא|טחון|מעושן|שניצל|בקר|הודו|כנף|מקוצב|פרימיום|קהילות|לנדא|אפוי|נקניק|מרק|בשקית|בגריל|צמחוני|מחפוד|חצוי|עוף הכפר|פרימה|אחדות`},
	{ID: "ground-beef", He: "בקר טחון", En: "Ground beef", Category: "בשר ועוף", Unit: UnitKg, DefaultQty: 1,
		Include: `בקר טחון|בשר טחון|טחון בקר`,
		Exclude: `עוף|הודו|טלה|כבש|צמחי|סויה|טופו|קציצ|המבורגר|שניצל|מוכן|מעובד|קפוא|בשמן`},
	{ID: "salmon", He: "סלמון", En: "Salmon", Category: "דגים", Unit: UnitKg, DefaultQty: 0.5,
		Include: `סלמון`, Exclude: `מעושן|כבוש|פסטרמה|רוטב|סלט|קוויאר|אפוי|מטוגן|שימור|בשמן|קפוא|פרוס`},
	{ID: "tuna", He: "טונה בשמן", En: "Canned tuna", Category: "שימורים", Unit: UnitUnit, DefaultQty: 1, PackLabel: `160|185|140`,
		Include: `טונה`, Exclude: `צהובה|סטייק|פילה|טרי|סלט|ממרח|פסטה|כריך|רוטב|מקלות|צ.?יפס|קציצ|פשטידה`},

	// ---------- dairy ----------
	{ID: "milk", He: "חלב", En: "Milk", Category: "מוצרי חלב", Unit: UnitUnit, DefaultQty: 1, PackLabel: `1ל|1 ליטר|1ליטר|בקרטון|בשקית`,
		Include: `^חלב`,
		Exclude: `שוקולד|בטעם|סויה|שקד|שיבולת|מרוכז|אבקה|ממותק|עוג|גבינ|לבן|חמאה|שמנת|קוקוס|קרמל|אפיה|חלבון|יוגורט|פרוביו|גליד|קפה|משקה`},
	{ID: "eggs", He: "ביצים 12", En: "Eggs (12)", Category: "מוצרי חלב", Unit: UnitUnit, DefaultQty: 1, PackLabel: `12`,
		Include: `ביצים`,
		Exclude: `אטריות|נודלס|פסטה|סלט|קינדר|מקושקש|טרופ|נוזלי|אבקה|חלבון|מלפפון|עוג|מסטיק|שוקולד|חומוס|חציל|כרובית|טחינה|קיש|בגט|מאפה`},
	{ID: "yellow-cheese", He: "גבינה צהובה 400 ג", En: "Yellow cheese (400 g)", Category: "מוצרי חלב", Unit: UnitUnit, DefaultQty: 1, PackLabel: `400`,
		Include: `גבינה צהובה|ג\. ?צהובה|גבינה 28%|גבינה 22%|גבינה 15%|גבינה 9%|פרוסות גבינה`,
		Exclude: `מגורד|צמחי|טבעוני|טונה|שעועית|אשכולית|גאודה|בולגרית|פטה|מקרוני|רביולי|רוטב|חטיף|פריכ|נמס|קרואסון|בגט|קיש`},
	{ID: "cottage", He: "קוטג׳", En: "Cottage cheese", Category: "מוצרי חלב", Unit: UnitUnit, DefaultQty: 1, PackLabel: `250`,
		Include: `קוטג`, Exclude: `צמחי|ממרח|בטעם|טונה|עוג|סלט|חמאה|שמנת|פירורים`},
	{ID: "white-cheese", He: "גבינה לבנה", En: "White cheese", Category: "מוצרי חלב", Unit: UnitUnit, DefaultQty: 1, PackLabel: `250`,
		Include: `גבינה לבנה|גבינה.*5%|גבינה.*9%`,
		Exclude: `צהובה|מלוחה|צמחי|בולגרית|פטה|קוטג|ממרח|שמנת|עוג|בגט|קיש`},
	{ID: "yogurt", He: "יוגורט", En: "Yogurt", Category: "מוצרי חלב", Unit: UnitUnit, DefaultQty: 1, PackLabel: `200|250`,
		Include: `יוגורט|יוגור`, Exclude: `בטעם|פרוביו|יווני|משקה|קפוא|גרנולה|דנונה|עוג|שתיה`},
	{ID: "sour-cream", He: "שמנת חמוצה", En: "Sour cream", Category: "מוצרי חלב", Unit: UnitUnit, DefaultQty: 1, PackLabel: `200`,
		Include: `שמנת חמוצה|שמנת.*חמוצ`, Exclude: `מתוקה|מטוגן|רוטב|עוג|בגט|קיש`},
	{ID: "butter", He: "חמאה", En: "Butter", Category: "מוצרי חלב", Unit: UnitUnit, DefaultQty: 1, PackLabel: `100`,
		Include: `חמאה`, Exclude: `בוטנים|שקדים|קקאו|מרגרינה|צמחי|בטעם|עוג|קוקוס|שוקולד|אפיה|נטול|כוסמין|קוקי|ביסקוויט|ופל`},
	{ID: "cream-cheese", He: "גבינת שמנת", En: "Cream cheese", Category: "מוצרי חלב", Unit: UnitUnit, DefaultQty: 1, PackLabel: `200`,
		Include: `גבינת שמנת|שמנת.*גבינ`, Exclude: `חמוצה|מתוקה|עוג|רוטב|צמחי|בגט|קיש`},

	// ---------- pantry ----------
	{ID: "pasta", He: "פסטה 500 ג", En: "Pasta (500 g)", Category: "יבשים", Unit: UnitUnit, DefaultQty: 1, PackLabel: `500`,
		Include: `פסטה|ספגטי|מקרוני|פוזילי|פטוציני|ריגטוני|פרפלה`,
		Exclude: `מוקפא|טרי|סלט|מנה|רוטב|גבינ|מילוי|רביולי|טורטליני|קפוא|אטריות|סיני|ביצים|מוקרם`},
	{ID: "rice", He: "אורז", En: "Rice", Category: "יבשים", Unit: UnitUnit, DefaultQty: 1, PackLabel: `1 ?ק|1000`,
		Include: `אורז`, Exclude: `מוקפץ|מנה|עוג|חטיף|אפוי|חלב|פודינג|קוקי|מטוגן`},
	{ID: "sugar", He: "סוכר", En: "Sugar", Category: "יבשים", Unit: UnitUnit, DefaultQty: 1, PackLabel: `1 ?ק|1000`,
		Include: `^סוכר`, Exclude: `מסטיק|עוג|ממתיק|דיאט|ואנילין|אבקה`},
	{ID: "flour", He: "קמח", En: "Flour", Category: "יבשים", Unit: UnitUnit, DefaultQty: 1, PackLabel: `1 ?ק|1000`,
		Include: `^קמח`, Exclude: `מטוגן|עוג|תערובת|מוכן|לחם|שקד|קוקוס|תירס|כוסמין|אפיה|פירור`},
	{ID: "oil", He: "שמן 1 ליטר", En: "Cooking oil (1 L)", Category: "יבשים", Unit: UnitUnit, DefaultQty: 1, PackLabel: `1 ?ל|1ליטר|1000`,
		Include: `שמן חמניות|שמן קנולה|שמן סויה|שמן תירס|שמן צמחי|שמן מזוקק`,
		Exclude: `זית|שומשום|קוקוס|אבוקדו|מנוע|גוף|שיער|אתרי|בשמן|שוקולד`},
	{ID: "olive-oil", He: "שמן זית", En: "Olive oil", Category: "יבשים", Unit: UnitUnit, DefaultQty: 1, PackLabel: `750|1 ?ל|500|1ליטר`,
		Include: `שמן זית`, Exclude: `בשמן|מטוגן|רוטב|עלים|כבוש|זיתים|קוסקוס|מגורד`},
	{ID: "bread", He: "לחם", En: "Bread", Category: "מאפים", Unit: UnitUnit, DefaultQty: 1, PackLabel: `אחיד|פרזנס|לחם`,
		Include: `לחם אחיד|לחם פרזנס|לחם לבן|לחם כהה|לחם מלא|לחם שיפון|לחם חמניות|לחם פרוס`,
		Exclude: `פירור|קמח|מטוגן|לחמניה|פיתה|טוסט|קרוטון|פריכ|עוג|מתוק|חלב|שום|בצל|קפוא|בגט|קיש|כריך|מאפה`},
	{ID: "hummus", He: "חומוס", En: "Hummus", Category: "מוצרים", Unit: UnitUnit, DefaultQty: 1, PackLabel: `250|400|500`,
		Include: `חומוס`, Exclude: `טחינה|חטיף|פיצוח|קמח|מטוגן|סלט|גרגיר|יבש|קלוי`},
	{ID: "tahini", He: "טחינה", En: "Tahini", Category: "מוצרים", Unit: UnitUnit, DefaultQty: 1, PackLabel: `500|250|400`,
		Include: `טחינה`, Exclude: `חלבה|גלידה|עוג|ממרח|חומוס|חטיף`},
	{ID: "coffee", He: "קפה נמס", En: "Instant coffee", Category: "משקאות", Unit: UnitUnit, DefaultQty: 1, PackLabel: `200|100`,
		Include: `קפה נמס|קפה טורקי|קפה שחור`,
		Exclude: `מכונה|מוכן|קר|חלב|בטעם|שוקולד|עוג|גליד|סוכר|כוסות|מכונ|בגט`},
	{ID: "cereal", He: "קורנפלקס", En: "Breakfast cereal", Category: "יבשים", Unit: UnitUnit, DefaultQty: 1, PackLabel: `500|375|700`,
		Include: `קורנפלקס|דגני בוקר|שיבולת שועל|גרנולה`,
		Exclude: `חטיף|עוג|קוקי|ביסקוויט|שוקולד|מטוגן|לחם|יוגורט|בגט|קיש`},
	{ID: "chocolate", He: "שוקולד", En: "Chocolate bar", Category: "חטיפים", Unit: UnitUnit, DefaultQty: 1, PackLabel: `100`,
		Include: `שוקולד`, Exclude: `ממרח|עוג|גליד|חטיף|ביסקוויט|קוקי|פודינג|שתיה|קרמל|אבקה|מטוגן|קרואסון|בגט`},
}

func init() {
	for i := range Items {
		it := &Items[i]
		it.inc = regexp.MustCompile(it.Include)
		if it.Exclude != "" {
			it.exc = regexp.MustCompile(it.Exclude)
		}
		if it.PackLabel != "" {
			it.pack = regexp.MustCompile(it.PackLabel)
		}
	}
}

// Match reports whether a chain product name is a candidate for this item.
func (it *Item) Match(name string) bool {
	if !it.inc.MatchString(name) {
		return false
	}
	return it.exc == nil || !it.exc.MatchString(name)
}

// MatchPack reports whether a packaged product carries an accepted pack size.
func (it *Item) MatchPack(name string) bool {
	return it.pack == nil || it.pack.MatchString(name)
}

// PreferScore ranks candidates of the same item so a standard pack wins.
// Higher is better. Weighted items score 0.
func (it *Item) PreferScore(name string, qty float64) int {
	if it.Unit == UnitKg {
		return 0
	}
	score := 0
	if it.pack != nil && it.pack.MatchString(name) {
		score += 10
	}
	if it.pack != nil {
		if m := it.pack.FindString(name); m != "" {
			score += len(m)
		}
	}
	return score
}
