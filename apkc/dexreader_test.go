package main

// The fixture is the real classes.dex that d8 9.2.4-dev wrote for the studio project, the one
// e2e.sh builds with the SDK of this phone. 2844 bytes, pinned as base64 because this repo does
// not version binaries. Its counts are the ones measured in docs/evidencia.md §13 and §14, so
// these tests are that measurement written down, and the reader below is what will name the
// section where our own output differs from d8's once the writer exists.

import (
	"encoding/base64"
	"strings"
	"testing"
)

const d8DexFixture = "ZGV4CjAzNQAb2qZhWXLqRBiKWWmdcUOBkG3WOTSsm4kcCwAAcAAAAHhWNBIAAAAAAAAAAEwKAAA1" +
	"AAAAcAAAABcAAABEAQAACQAAAKABAAAEAAAADAIAABEAAAAsAgAABgAAALQCAACoBwAAdAMAAOoE" +
	"AADyBAAA/AQAAAYFAAAUBQAAFwUAABsFAAAhBQAAOQUAAFQFAABpBQAAfgUAAK8FAADIBQAA4AUA" +
	"AAYGAAAqBgAARgYAAGYGAACFBgAAngYAAMIGAADnBgAABwcAACoHAABQBwAAagcAAH4HAACRBwAA" +
	"mQcAAJwHAACgBwAApAcAAKcHAACrBwAAuAcAAL0HAADBBwAAyQcAAM8HAADZBwAA3wcAAOYHAADw" +
	"BwAABQgAABYIAAAeCAAALggAAEoIAABQCAAAVwgAAF8IAABmCAAABAAAAAcAAAAIAAAACQAAAAoA" +
	"AAALAAAADAAAAA0AAAAOAAAADwAAABAAAAARAAAAEgAAABMAAAAUAAAAFQAAABYAAAAXAAAAGAAA" +
	"ABkAAAAaAAAAHQAAACAAAAAFAAAABgAAAKgEAAAFAAAABgAAALAEAAAGAAAABwAAALgEAAAdAAAA" +
	"FQAAAAAAAAAeAAAAFQAAAMQEAAAfAAAAFQAAAMwEAAAfAAAAFQAAANQEAAAhAAAAFgAAANwEAAAh" +
	"AAAAFgAAAOQEAAAIAAkAMgAAAAoAAAAtAAAACwAAACYAAAAMAAAAAQAAAAEAAwAAAAAAAQAFACoA" +
	"AAAEAAEAIwAAAAYAAAAvAAAABwACACcAAAAHAAMAMAAAAAgABgAAAAAACAAIACwAAAAJAAMAAAAA" +
	"AAkABQAqAAAACQAHACsAAAAJAAQALgAAAAoAAwAAAAAACwADAAAAAAAMAAMAAAAAAA0AAwAAAAAA" +
	"FAADAAAAAAAIAAAAAAAAABQAAACoBAAAGwAAAPQJAABQCQAAAAAAAAkAAAABAAAAAQAAAAAAAAAb" +
	"AAAAAAAAAGEJAAAAAAAACgAAABEAAAAUAAAAAAAAABwAAAAMCgAAcwkAAKEJAAALAAAAEQAAABQA" +
	"AAAAAAAAHAAAABwKAAB/CQAApwkAAAwAAAARAAAAFAAAAAAAAAAcAAAALAoAAIsJAACtCQAADQAA" +
	"ABEAAAAUAAAAAAAAABwAAAA8CgAAlwkAAAAAAAAEAAIAAwAAAIgEAAAOAAAAVCMAABoAAgASAXEw" +
	"BAADAQwDbhAFAAMAEhMPAwIAAgABAAAAjQQAAAYAAABbAQAAcBAQAAAADgADAAIAAgAAAI0EAAAQ" +
	"AAAAGgADAHIgAgACAAwCIgAIAHAgBgAQAHIgAwACABISDwIBAAEAAQAAAJIEAAAEAAAAcBAAAAAA" +
	"DgACAAIAAgAAAJYEAAAJAAAAbyABABAAFQECf24gCwAQAA4AAAABAAEAAQAAAJIEAAAEAAAAcBAQ" +
	"AAAADgABAAEAAQAAAJwEAAAEAAAAcBAQAAAADgABAAEAAQAAAKAEAAAEAAAAcBAQAAAADgABAAEA" +
	"AQAAAKQEAAAEAAAAcBAQAAAADgAWAQAOABQBAA4ACwAOAA4BAA5aAA4ADgARAA4ACgAOAAEAAAAF" +
	"AAAAAQAAABMAAAADAAAAAgATAAAAAAABAAAAAAAAAAEAAAADAAAAAQAAAAkAAAABAAAABAAAAAEA" +
	"AAAGAAY8aW5pdD4ACEFwcFRoZW1lAAhDbGlja2VkIQAMSGVsbG8gV29ybGQhAAFJAAJMTAAETExM" +
	"SQAWTGFuZHJvaWQvYXBwL0FjdGl2aXR5OwAZTGFuZHJvaWQvY29udGVudC9Db250ZXh0OwATTGFu" +
	"ZHJvaWQvb3MvQnVuZGxlOwATTGFuZHJvaWQvdmlldy9NZW51OwAvTGFuZHJvaWQvdmlldy9NZW51" +
	"SXRlbSRPbk1lbnVJdGVtQ2xpY2tMaXN0ZW5lcjsAF0xhbmRyb2lkL3ZpZXcvTWVudUl0ZW07ABZM" +
	"YW5kcm9pZC93aWRnZXQvVG9hc3Q7ACRMY29tL2hvbGEvYXBrY3N0dWRpby9NYWluQWN0aXZpdHkk" +
	"MTsAIkxjb20vaG9sYS9hcGtjc3R1ZGlvL01haW5BY3Rpdml0eTsAGkxjb20vaG9sYS9hcGtjc3R1" +
	"ZGlvL1IkaWQ7AB5MY29tL2hvbGEvYXBrY3N0dWRpby9SJGxheW91dDsAHUxjb20vaG9sYS9hcGtj" +
	"c3R1ZGlvL1Ikc3R5bGU7ABdMY29tL2hvbGEvYXBrY3N0dWRpby9SOwAiTGRhbHZpay9hbm5vdGF0" +
	"aW9uL0VuY2xvc2luZ0NsYXNzOwAjTGRhbHZpay9hbm5vdGF0aW9uL0VuY2xvc2luZ01ldGhvZDsA" +
	"HkxkYWx2aWsvYW5ub3RhdGlvbi9Jbm5lckNsYXNzOwAhTGRhbHZpay9hbm5vdGF0aW9uL01lbWJl" +
	"ckNsYXNzZXM7ACRMZGFsdmlrL2Fubm90YXRpb24vTWV0aG9kUGFyYW1ldGVyczsAGExqYXZhL2xh" +
	"bmcvQ2hhclNlcXVlbmNlOwASTGphdmEvbGFuZy9PYmplY3Q7ABFNYWluQWN0aXZpdHkuamF2YQAG" +
	"Ui5qYXZhAAFWAAJWSQACVkwAAVoAAlpMAAthY2Nlc3NGbGFncwADYWRkAAJpZAAGbGF5b3V0AARt" +
	"YWluAAhtYWtlVGV4dAAEbmFtZQAFbmFtZXMACG9uQ3JlYXRlABNvbkNyZWF0ZU9wdGlvbnNNZW51" +
	"AA9vbk1lbnVJdGVtQ2xpY2sABnB1bHNhcgAOc2V0Q29udGVudFZpZXcAGnNldE9uTWVudUl0ZW1D" +
	"bGlja0xpc3RlbmVyAARzaG93AAVzdHlsZQAGdGhpcyQwAAV2YWx1ZQCeAX5+RDh7ImJhY2tlbmQi" +
	"OiJkZXgiLCJjb21waWxhdGlvbi1tb2RlIjoicmVsZWFzZSIsImhhcy1jaGVja3N1bXMiOmZhbHNl" +
	"LCJtaW4tYXBpIjoyMSwic2hhLTEiOiIwODRhODkxMjZhZTA1OTZjNTk5MTQzZGY1N2E2NTQ2NDZh" +
	"ZjI4MzEzIiwidmVyc2lvbiI6IjkuMi40LWRldiJ9AAISAiIcAUQQgAApHAEeAg8BMxoKAhACIgQA" +
	"KB4CEAIiBBkoFyQCDgEzGA0CEAIiBBkoFyUCEAIiBBkoFzECEQEzHAMYDBgLGAoAAQEBAJAgBoCA" +
	"BKAHBwH0BgAAAQIIgYAE7AcJAYQIAQG8BwEAAQABGQyBgASoCAEAAQACGQ2BgATACAEAAQADGQ6B" +
	"gATYCAAAAQAPgYAE8AgBZAAAAX8BZAAAAn8BZAAAA38AAQAAAAcJAAACAAAAFQkAABsJAAACAAAA" +
	"LAkAACMJAAACAAAALAkAADIJAAACAAAALAkAADsJAAABAAAARAkAALwJAAAAAAAAAQAAAAAAAAAG" +
	"AAAAtAkAAMgJAAAAAAAAAAAAAAAAAADUCQAAAAAAAAAAAAAAAAAA4AkAAAAAAAAAAAAAAAAAAOwJ" +
	"AAAAAAAAAAAAAAAAAAARAAAAAAAAAAEAAAAAAAAAAQAAADUAAABwAAAAAgAAABcAAABEAQAAAwAA" +
	"AAkAAACgAQAABAAAAAQAAAAMAgAABQAAABEAAAAsAgAABgAAAAYAAAC0AgAAASAAAAkAAAB0AwAA" +
	"AyAAAAcAAACIBAAAARAAAAgAAACoBAAAAiAAADUAAADqBAAABCAAAAgAAAAHCQAAACAAAAYAAABQ" +
	"CQAABSAAAAMAAAChCQAAAxAAAAYAAAC0CQAABiAAAAUAAAD0CQAAABAAAAEAAABMCgAA" +
	""

func fixtureDex(t *testing.T) []byte {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString(d8DexFixture)
	if err != nil {
		t.Fatalf("the fixture is not the base64 it claims: %v", err)
	}
	if len(b) != 2844 {
		t.Fatalf("the fixture is %d bytes, the measured dex was 2844", len(b))
	}
	return b
}

func TestReadDexReadsWhatD8Wrote(t *testing.T) {
	d, err := readDex(fixtureDex(t))
	if err != nil {
		t.Fatalf("readDex on a dex d8 wrote: %v", err)
	}
	for _, e := range []struct {
		name string
		got  int
		want uint32
	}{
		{"string_ids", len(d.strings), 53},
		{"type_ids", len(d.types), 23},
		{"proto_ids", len(d.protos), 9},
		{"field_ids", len(d.fields), 4},
		{"method_ids", len(d.methods), 17},
		{"class_defs", len(d.classes), 6},
	} {
		if uint32(e.got) != e.want {
			t.Errorf("%s: the reader found %d, d8 wrote %d", e.name, e.got, e.want)
		}
	}
	if len(d.maps) != 17 {
		t.Errorf("the map_list has %d entries, the measured one had 17", len(d.maps))
	}
	if d.version != "035" {
		t.Errorf("dex version %q, the studio project asks for 035", d.version)
	}
}

func TestReadDexResolvesNamesNotJustIndexes(t *testing.T) {
	d, err := readDex(fixtureDex(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Landroid/app/Activity;",
		"Lcom/hola/apkcstudio/MainActivity;",
		"Lcom/hola/apkcstudio/R$id;",
		"Ljava/lang/Object;",
	} {
		found := false
		for _, s := range d.types {
			if s == want {
				found = true
			}
		}
		if !found {
			t.Errorf("%s is not among the %d type descriptors the reader resolved", want, len(d.types))
		}
	}

	names := map[string]int{}
	for _, m := range d.methods {
		names[m.name]++
	}
	for _, want := range []string{"<init>", "onCreate", "setContentView", "makeText", "show", "onMenuItemClick"} {
		if names[want] == 0 {
			t.Errorf("no method named %q; the reader resolved %v", want, names)
		}
	}

	// the marker d8 leaves in the pool: if the reader stops before the last strings, this is the
	// first thing that goes missing
	if len(d.stringsOf("~~D8")) != 1 {
		t.Errorf("the ~~D8 metadata string is missing, found %v", d.stringsOf("~~D8"))
	}
}

func TestReadDexAddsUpToTheMeasuredSubset(t *testing.T) {
	d, err := readDex(fixtureDex(t))
	if err != nil {
		t.Fatal(err)
	}
	// docs/evidencia.md §14 counted 9 methods with Code in javap over the six classes
	total := 0
	for _, c := range d.classes {
		total += c.directs + c.virtuals
	}
	if total != 9 {
		t.Errorf("%d methods in the class_data items, javap counted 9 with Code", total)
	}
}

func TestDexIntegrityAcceptsWhatD8Wrote(t *testing.T) {
	d, err := readDex(fixtureDex(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Integrity(); err != nil {
		t.Errorf("the real dex fails its own checksum and signature: %v", err)
	}
}

// TestDexIntegrityNoticesOneByteChanged is the difference between a reader and a checker: the
// header of a dex carries an adler32 over its own bytes and a sha-1 over everything after the
// signature, so a file that parsed can still be a file the installer will refuse.
func TestDexIntegrityNoticesOneByteChanged(t *testing.T) {
	b := fixtureDex(t)
	d, err := readDex(b)
	if err != nil {
		t.Fatal(err)
	}
	at := d.tableOff["data"][1] + 40
	tampered := append([]byte{}, b...)
	tampered[at] ^= 0xFF

	victim, err := readDex(tampered)
	if err != nil {
		// the string data itself may break parsing, which is also a refusal: what must not
		// happen is accepting it
		return
	}
	if err := victim.Integrity(); err == nil {
		t.Fatal("Integrity accepted a dex whose bytes no longer match its header")
	} else if !strings.Contains(err.Error(), "adler32") {
		t.Errorf("the refusal says %q, and it has to name the checksum that broke", err)
	}
}

func TestReadDexRefusesSomethingThatIsNotADex(t *testing.T) {
	for _, b := range [][]byte{[]byte("this is not a dex file at all"), {0x50, 0x4b, 0x03, 0x04}} {
		if _, err := readDex(b); err == nil {
			t.Errorf("%q was accepted as a dex", b[:4])
		} else if !strings.Contains(err.Error(), "dex") {
			t.Errorf("the refusal says %q, and it has to say why this is not a dex", err)
		}
	}
}

func TestDecodeMUTF8SpellsTheTwoThingsUTF8DoesNot(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want string
	}{
		{"ascii", []byte{0x41, 0x42}, "AB"},
		// U+0000 is written C0 80, which is the whole point of the form: no terminator can
		// appear inside a string
		{"nul", []byte{0xC0, 0x80}, "\x00"},
		{"two bytes", []byte{0xC3, 0xA9}, "é"},
		{"two bytes, not nul", []byte{0xC2, 0xA9}, "©"},
		{"three bytes", []byte{0xE2, 0x82, 0xAC}, "€"},
		// a character outside the plane arrives as two three-byte surrogates (CESU-8)
		{"surrogate pair", []byte{0xED, 0xA0, 0xBD, 0xED, 0xB8, 0x80}, "\U0001F600"},
	}
	for _, c := range cases {
		got, err := decodeMUTF8(c.in)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s: decoded %q, want %q", c.name, got, c.want)
		}
	}
	if n := utf16Len("a\U0001F600b"); n != 4 {
		t.Errorf("utf16Len counted %d for a-b around one astral character, want 4", n)
	}
	if _, err := decodeMUTF8([]byte{0xE2, 0x82}); err == nil {
		t.Error("a truncated three byte form was accepted")
	}
}
