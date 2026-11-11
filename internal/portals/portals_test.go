package portals

import (
	"bytes"
	"compress/gzip"
	"testing"
	"unicode/utf16"
)

func utf16leBytes(s string) []byte {
	u := utf16.Encode([]rune(s))
	out := []byte{0xff, 0xfe}
	for _, r := range u {
		out = append(out, byte(r), byte(r>>8))
	}
	return out
}

func TestDecodeUTF16LE(t *testing.T) {
	xml := `<Root><ChainName>רמי לוי</ChainName></Root>`
	got, err := Decode(utf16leBytes(xml))
	if err != nil {
		t.Fatal(err)
	}
	if got != xml {
		t.Fatalf("got %q want %q", got, xml)
	}
}

func TestDecodeUTF8BOM(t *testing.T) {
	xml := `<Root>ok</Root>`
	got, _ := Decode(append([]byte{0xef, 0xbb, 0xbf}, []byte(xml)...))
	if got != xml {
		t.Fatalf("got %q", got)
	}
}

func TestDecodeGzipUTF16(t *testing.T) {
	xml := `<Root><Store><StoreID>001</StoreID></Store></Root>`
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	zw.Write(utf16leBytes(xml))
	zw.Close()
	got, err := Decode(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if got != xml {
		t.Fatalf("got %q", got)
	}
}

const sampleStores = `<Root><ChainID>7290058140886</ChainID><ChainName>רמי לוי שיווק השקמה</ChainName>
<SubChains><SubChain><SubChainID>001</SubChainID><SubChainName>1</SubChainName><Stores>
<Store><StoreID>016</StoreID><StoreName>רמי לוי רמת גן</StoreName><Address>הרצל 100</Address><City>רמת גן</City></Store>
<Store><StoreID>040</StoreID><StoreName>רמי לוי ירושלים</StoreName><Address>יפו 5 &amp; 7</Address><City>ירושלים</City></Store>
</Stores></SubChain></SubChains></Root>`

func TestParseStores(t *testing.T) {
	stores := ParseStores(sampleStores)
	if len(stores) != 2 {
		t.Fatalf("got %d stores", len(stores))
	}
	if stores[0].ID != "16" {
		t.Errorf("leading zeros not stripped: %q", stores[0].ID)
	}
	if stores[1].Address != "יפו 5 & 7" {
		t.Errorf("entity not unescaped: %q", stores[1].Address)
	}
	if stores[1].City != "ירושלים" {
		t.Errorf("city: %q", stores[1].City)
	}
}

const sampleItems = `<Root><Items>
<Item><ItemCode>1</ItemCode><ItemName>מלפפון</ItemName><ItemPrice>4.90</ItemPrice><Quantity>1.000</Quantity><UnitOfMeasure>קילוגרם</UnitOfMeasure><bIsWeighted>1</bIsWeighted><UnitOfMeasurePrice>4.90</UnitOfMeasurePrice></Item>
<Item><ItemCode>2</ItemCode><ItemName>חלב 3% בקרטון</ItemName><ItemPrice>6.20</ItemPrice><Quantity>1.000</Quantity><UnitOfMeasure>ליטר</UnitOfMeasure><bIsWeighted>0</bIsWeighted></Item>
<Item><ItemCode>3</ItemCode><ItemName>לא במבצע</ItemName><ItemPrice>0</ItemPrice><Quantity>1</Quantity></Item>
</Items></Root>`

func TestParseItems(t *testing.T) {
	items := ParseItems(sampleItems)
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2 (zero price skipped)", len(items))
	}
	if !items[0].Weighted || items[0].Price != 4.90 {
		t.Errorf("weighted item parsed wrong: %+v", items[0])
	}
	if items[1].Weighted {
		t.Errorf("plain item marked weighted")
	}
}
