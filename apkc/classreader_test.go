package main

// Fixtures for the class reader: the six .class files javac wrote for the studio project, pinned as
// base64 so the test needs no JDK, plus the numbers the oracle printed. Every stack, local count,
// code length and instruction count below is what javap -v -c said over these same files on
// 2026-10-27; docs/evidencia.md §17 has the script that measured them. The value of R$id.pulsar is
// the resource id aapt2 assigned, which also appears in the R.java the build generated.

import (
	"encoding/base64"
	"strings"
	"testing"
)

// com/hola/apkcstudio/MainActivity$1.class -- 900 bytes, tal como javac lo dej6 en build/classes
const clase1Fixture = "yv66vgAAAEEALQkAAgADBwAEDAAFAAYBACJjb20vaG9sYS9hcGtjc3R1ZGlvL01haW5BY3Rpdml0" +
	"eSQxAQAGdGhpcyQwAQAiTGNvbS9ob2xhL2Fwa2NzdHVkaW8vTWFpbkFjdGl2aXR5OwoACAAJBwAK" +
	"DAALAAwBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCAAOAQAIQ2xpY2tlZCEHABAB" +
	"ABRhbmRyb2lkL3dpZGdldC9Ub2FzdAoADwASDAATABQBAAhtYWtlVGV4dAEASihMYW5kcm9pZC9j" +
	"b250ZW50L0NvbnRleHQ7TGphdmEvbGFuZy9DaGFyU2VxdWVuY2U7SSlMYW5kcm9pZC93aWRnZXQv" +
	"VG9hc3Q7CgAPABYMABcADAEABHNob3cHABkBAC1hbmRyb2lkL3ZpZXcvTWVudUl0ZW0kT25NZW51" +
	"SXRlbUNsaWNrTGlzdGVuZXIBACUoTGNvbS9ob2xhL2Fwa2NzdHVkaW8vTWFpbkFjdGl2aXR5OylW" +
	"AQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEAEE1ldGhvZFBhcmFtZXRlcnMBAA9vbk1lbnVJdGVt" +
	"Q2xpY2sBABooTGFuZHJvaWQvdmlldy9NZW51SXRlbTspWgEAClNvdXJjZUZpbGUBABFNYWluQWN0" +
	"aXZpdHkuamF2YQEAD0VuY2xvc2luZ01ldGhvZAcAJAEAIGNvbS9ob2xhL2Fwa2NzdHVkaW8vTWFp" +
	"bkFjdGl2aXR5DAAmACcBABNvbkNyZWF0ZU9wdGlvbnNNZW51AQAWKExhbmRyb2lkL3ZpZXcvTWVu" +
	"dTspWgEACE5lc3RIb3N0AQAMSW5uZXJDbGFzc2VzBwArAQAVYW5kcm9pZC92aWV3L01lbnVJdGVt" +
	"AQAXT25NZW51SXRlbUNsaWNrTGlzdGVuZXIAIAACAAgAAQAYAAEQEAAFAAYAAAACAAAACwAaAAIA" +
	"GwAAACIAAgACAAAACiortQABKrcAB7EAAAABABwAAAAGAAEAAAAUAB0AAAAFAQAAgBAAAQAeAB8A" +
	"AQAbAAAAKwADAAIAAAAPKrQAARINA7gAEbYAFQSsAAAAAQAcAAAACgACAAAAFgANABcABAAgAAAA" +
	"AgAhACIAAAAEACMAJQAoAAAAAgAjACkAAAASAAIAAgAAAAAAAAAYACoALAYJ" +
	""

// com/hola/apkcstudio/MainActivity.class -- 1036 bytes, tal como javac lo dej6 en build/classes
const clase2Fixture = "yv66vgAAAEEANQoAAgADBwAEDAAFAAYBABRhbmRyb2lkL2FwcC9BY3Rpdml0eQEABjxpbml0PgEA" +
	"AygpVgoAAgAIDAAJAAoBAAhvbkNyZWF0ZQEAFihMYW5kcm9pZC9vcy9CdW5kbGU7KVYHAAwBABxj" +
	"b20vaG9sYS9hcGtjc3R1ZGlvL1IkbGF5b3V0A38CAAAKAA8AEAcAEQwAEgATAQAgY29tL2hvbGEv" +
	"YXBrY3N0dWRpby9NYWluQWN0aXZpdHkBAA5zZXRDb250ZW50VmlldwEABChJKVYIABUBAAxIZWxs" +
	"byBXb3JsZCELABcAGAcAGQwAGgAbAQARYW5kcm9pZC92aWV3L01lbnUBAANhZGQBADEoTGphdmEv" +
	"bGFuZy9DaGFyU2VxdWVuY2U7KUxhbmRyb2lkL3ZpZXcvTWVudUl0ZW07BwAdAQAiY29tL2hvbGEv" +
	"YXBrY3N0dWRpby9NYWluQWN0aXZpdHkkMQoAHAAfDAAFACABACUoTGNvbS9ob2xhL2Fwa2NzdHVk" +
	"aW8vTWFpbkFjdGl2aXR5OylWCwAiACMHACQMACUAJgEAFWFuZHJvaWQvdmlldy9NZW51SXRlbQEA" +
	"GnNldE9uTWVudUl0ZW1DbGlja0xpc3RlbmVyAQBIKExhbmRyb2lkL3ZpZXcvTWVudUl0ZW0kT25N" +
	"ZW51SXRlbUNsaWNrTGlzdGVuZXI7KUxhbmRyb2lkL3ZpZXcvTWVudUl0ZW07AQAEQ29kZQEAD0xp" +
	"bmVOdW1iZXJUYWJsZQEAE29uQ3JlYXRlT3B0aW9uc01lbnUBABYoTGFuZHJvaWQvdmlldy9NZW51" +
	"OylaAQAKU291cmNlRmlsZQEAEU1haW5BY3Rpdml0eS5qYXZhAQALTmVzdE1lbWJlcnMBAAxJbm5l" +
	"ckNsYXNzZXMHADABABVjb20vaG9sYS9hcGtjc3R1ZGlvL1IBAAZsYXlvdXQHADMBAC1hbmRyb2lk" +
	"L3ZpZXcvTWVudUl0ZW0kT25NZW51SXRlbUNsaWNrTGlzdGVuZXIBABdPbk1lbnVJdGVtQ2xpY2tM" +
	"aXN0ZW5lcgAhAA8AAgAAAAAAAwABAAUABgABACcAAAAdAAEAAQAAAAUqtwABsQAAAAEAKAAAAAYA" +
	"AQAAAAsAAQAJAAoAAQAnAAAALAACAAIAAAAMKiu3AAcqEg22AA6xAAAAAQAoAAAADgADAAAADgAF" +
	"AA8ACwAQAAEAKQAqAAEAJwAAADQABAACAAAAGCsSFLkAFgIAuwAcWSq3AB65ACECAFcErAAAAAEA" +
	"KAAAAAoAAgAAABQAFgAaAAMAKwAAAAIALAAtAAAABAABABwALgAAABoAAwALAC8AMQAZABwAAAAA" +
	"AAAAMgAiADQGCQ==" +
	""

// com/hola/apkcstudio/R$id.class -- 331 bytes, tal como javac lo dej6 en build/classes
const clase3Fixture = "yv66vgAAAEEAFgoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClW" +
	"BwAIAQAYY29tL2hvbGEvYXBrY3N0dWRpby9SJGlkAQAGcHVsc2FyAQABSQEADUNvbnN0YW50VmFs" +
	"dWUDfwEAAAEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBAApTb3VyY2VGaWxlAQAGUi5qYXZhAQAI" +
	"TmVzdEhvc3QHABMBABVjb20vaG9sYS9hcGtjc3R1ZGlvL1IBAAxJbm5lckNsYXNzZXMBAAJpZAAx" +
	"AAcAAgAAAAEAGQAJAAoAAQALAAAAAgAMAAEAAQAFAAYAAQANAAAAHQABAAEAAAAFKrcAAbEAAAAB" +
	"AA4AAAAGAAEAAAALAAMADwAAAAIAEAARAAAAAgASABQAAAAKAAEABwASABUAGQ==" +
	""

// com/hola/apkcstudio/R$layout.class -- 337 bytes, tal como javac lo dej6 en build/classes
const clase4Fixture = "yv66vgAAAEEAFgoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClW" +
	"BwAIAQAcY29tL2hvbGEvYXBrY3N0dWRpby9SJGxheW91dAEABG1haW4BAAFJAQANQ29uc3RhbnRW" +
	"YWx1ZQN/AgAAAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEAClNvdXJjZUZpbGUBAAZSLmphdmEB" +
	"AAhOZXN0SG9zdAcAEwEAFWNvbS9ob2xhL2Fwa2NzdHVkaW8vUgEADElubmVyQ2xhc3NlcwEABmxh" +
	"eW91dAAxAAcAAgAAAAEAGQAJAAoAAQALAAAAAgAMAAEAAQAFAAYAAQANAAAAHQABAAEAAAAFKrcA" +
	"AbEAAAABAA4AAAAGAAEAAAAOAAMADwAAAAIAEAARAAAAAgASABQAAAAKAAEABwASABUAGQ==" +
	""

// com/hola/apkcstudio/R$style.class -- 339 bytes, tal como javac lo dej6 en build/classes
const clase5Fixture = "yv66vgAAAEEAFgoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClW" +
	"BwAIAQAbY29tL2hvbGEvYXBrY3N0dWRpby9SJHN0eWxlAQAIQXBwVGhlbWUBAAFJAQANQ29uc3Rh" +
	"bnRWYWx1ZQN/AwAAAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEAClNvdXJjZUZpbGUBAAZSLmph" +
	"dmEBAAhOZXN0SG9zdAcAEwEAFWNvbS9ob2xhL2Fwa2NzdHVkaW8vUgEADElubmVyQ2xhc3NlcwEA" +
	"BXN0eWxlADEABwACAAAAAQAZAAkACgABAAsAAAACAAwAAQABAAUABgABAA0AAAAdAAEAAQAAAAUq" +
	"twABsQAAAAEADgAAAAYAAQAAABEAAwAPAAAAAgAQABEAAAACABIAFAAAAAoAAQAHABIAFQAZ" +
	""

// com/hola/apkcstudio/R.class -- 390 bytes, tal como javac lo dej6 en build/classes
const clase6Fixture = "yv66vgAAAEEAGAoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClW" +
	"BwAIAQAVY29tL2hvbGEvYXBrY3N0dWRpby9SAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEAClNv" +
	"dXJjZUZpbGUBAAZSLmphdmEBAAtOZXN0TWVtYmVycwcADwEAG2NvbS9ob2xhL2Fwa2NzdHVkaW8v" +
	"UiRzdHlsZQcAEQEAHGNvbS9ob2xhL2Fwa2NzdHVkaW8vUiRsYXlvdXQHABMBABhjb20vaG9sYS9h" +
	"cGtjc3R1ZGlvL1IkaWQBAAxJbm5lckNsYXNzZXMBAAVzdHlsZQEABmxheW91dAEAAmlkADEABwAC" +
	"AAAAAAABAAEABQAGAAEACQAAAB0AAQABAAAABSq3AAGxAAAAAQAKAAAABgABAAAACgADAAsAAAAC" +
	"AAwADQAAAAgAAwAOABAAEgAUAAAAGgADAA4ABwAVABkAEAAHABYAGQASAAcAFwAZ" +
	""

func leeLasSeis(t *testing.T) []classFull {
	t.Helper()
	fixtures := []string{clase1Fixture, clase2Fixture, clase3Fixture, clase4Fixture, clase5Fixture, clase6Fixture}
	out := []classFull{}
	for i, f := range fixtures {
		b, err := base64.StdEncoding.DecodeString(f)
		if err != nil {
			t.Fatalf("fixture %d is not base64: %v", i+1, err)
		}
		cf, err := readClassFull(b)
		if err != nil {
			t.Fatalf("fixture %d did not read: %v", i+1, err)
		}
		out = append(out, cf)
	}
	return out
}

func TestReadClassFullNamesTheSameSixClasses(t *testing.T) {
	clases := leeLasSeis(t)
	nombres := []string{
		"Lcom/hola/apkcstudio/MainActivity$1;",
		"Lcom/hola/apkcstudio/MainActivity;",
		"Lcom/hola/apkcstudio/R$id;",
		"Lcom/hola/apkcstudio/R$layout;",
		"Lcom/hola/apkcstudio/R$style;",
		"Lcom/hola/apkcstudio/R;",
	}
	if len(clases) != len(nombres) {
		t.Fatalf("%d clases leidas", len(clases))
	}
	for i, cf := range clases {
		if cf.Name != nombres[i] {
			t.Errorf("clase %d: %s, want %s", i+1, cf.Name, nombres[i])
		}
		// javap -v over these same files prints "major version: 65": the JDK of this phone
		// compiles to 21 unless the project asks otherwise, and d8 is what turns around
		if cf.Major != 65 {
			t.Errorf("%s: major version %d, javap says 65", cf.Name, cf.Major)
		}
	}

	main := clases[1]
	if main.Super != "Landroid/app/Activity;" {
		t.Errorf("MainActivity extends %s", main.Super)
	}
	if main.Access != accPublic|accSuper {
		t.Errorf("MainActivity access %#04x, javap printed 0x0021 (ACC_PUBLIC, ACC_SUPER)", main.Access)
	}
	if main.SourceFile != "MainActivity.java" {
		t.Errorf("SourceFile %q", main.SourceFile)
	}

	anona := clases[0]
	if len(anona.Interfaces) != 1 || anona.Interfaces[0] != "Landroid/view/MenuItem$OnMenuItemClickListener;" {
		t.Errorf("the anonymous class implements %v", anona.Interfaces)
	}
	if anona.Access != accSuper {
		t.Errorf("anonymous access %#04x, javap printed 0x0020 (ACC_SUPER)", anona.Access)
	}
	if len(anona.Fields) != 1 || anona.Fields[0].Name != "this$0" {
		t.Fatalf("fields of the anonymous class: %+v", anona.Fields)
	}
	if f := anona.Fields[0]; f.Access != accFinal|accSynthetic || f.Desc != "Lcom/hola/apkcstudio/MainActivity;" {
		t.Errorf("this$0 access %#04x desc %s, javap said 0x1010 ACC_FINAL, ACC_SYNTHETIC", f.Access, f.Desc)
	}

	if len(clases[2].Fields) != 1 || clases[2].Fields[0].Name != "pulsar" {
		t.Fatalf("R$id fields: %+v", clases[2].Fields)
	}
	if pulsar := clases[2].Fields[0]; !pulsar.HasConst || pulsar.ConstTag != cpInteger || pulsar.ConstInt != 2130771968 {
		t.Errorf("R$id.pulsar = %+v, and the id aapt2 gave is 0x7f010000 (2130771968)", pulsar)
	}
}

// TestReadClassFullAnswersWhatJavapCounted is the reader checked against the oracle rather than
// against itself: stack, locals, the length of the Code array and the instruction count of all nine
// methods that have code.
func TestReadClassFullAnswersWhatJavapCounted(t *testing.T) {
	clases := leeLasSeis(t)
	medido := []struct {
		clase, nombre, desc      string
		stack, locals, code, ins int
	}{
		{"Lcom/hola/apkcstudio/MainActivity$1;", "<init>", "(Lcom/hola/apkcstudio/MainActivity;)V", 2, 2, 10, 6},
		{"Lcom/hola/apkcstudio/MainActivity$1;", "onMenuItemClick", "(Landroid/view/MenuItem;)Z", 3, 2, 15, 8},
		{"Lcom/hola/apkcstudio/MainActivity;", "<init>", "()V", 1, 1, 5, 3},
		{"Lcom/hola/apkcstudio/MainActivity;", "onCreate", "(Landroid/os/Bundle;)V", 2, 2, 12, 7},
		{"Lcom/hola/apkcstudio/MainActivity;", "onCreateOptionsMenu", "(Landroid/view/Menu;)Z", 4, 2, 24, 11},
		{"Lcom/hola/apkcstudio/R$id;", "<init>", "()V", 1, 1, 5, 3},
		{"Lcom/hola/apkcstudio/R$layout;", "<init>", "()V", 1, 1, 5, 3},
		{"Lcom/hola/apkcstudio/R$style;", "<init>", "()V", 1, 1, 5, 3},
		{"Lcom/hola/apkcstudio/R;", "<init>", "()V", 1, 1, 5, 3},
	}

	conCode, totalIns := 0, 0
	for _, cf := range clases {
		for _, m := range cf.Methods {
			if !m.HasCode {
				continue
			}
			conCode++
			n, err := countJvmInstructions(m.Code)
			if err != nil {
				t.Errorf("%s%s of %s: %v", cf.Name, m.Name, m.Desc, err)
				continue
			}
			totalIns += n
		}
	}
	if conCode != len(medido) {
		t.Errorf("%d methods with Code, javap listed %d", conCode, len(medido))
	}
	if totalIns != 47 {
		t.Errorf("%d instructions walked, javap lists 47 over these six classes", totalIns)
	}

	for _, q := range medido {
		encontrado := false
		for _, cf := range clases {
			if cf.Name != q.clase {
				continue
			}
			for _, m := range cf.Methods {
				if m.Name != q.nombre || m.Desc != q.desc {
					continue
				}
				encontrado = true
				if m.MaxStack != q.stack || m.LocalCount != q.locals || len(m.Code) != q.code {
					t.Errorf("%s%s: stack=%d locals=%d code=%d, javap says %d/%d/%d",
						q.clase, q.nombre, m.MaxStack, m.LocalCount, len(m.Code), q.stack, q.locals, q.code)
				}
				if n, err := countJvmInstructions(m.Code); err != nil || n != q.ins {
					t.Errorf("%s%s: %d instructions walked (err %v), javap lists %d", q.clase, q.nombre, n, err, q.ins)
				}
			}
		}
		if !encontrado {
			t.Errorf("%s%s was not among the classes the reader returned", q.clase, q.nombre)
		}
	}
}

// TestArgCountIsTheSlotsTheParametersTake: a dex code_item wants args_size, and javap printed it as
// args=N. A long or a double takes two slots, and the receiver is one unless the method is static,
// which is why the anonymous class constructor reports 2 for a descriptor with one parameter.
func TestArgCountIsTheSlotsTheParametersTake(t *testing.T) {
	for _, c := range []struct {
		desc   string
		static bool
		want   int
	}{
		{"()V", false, 1},
		{"(Landroid/os/Bundle;)V", false, 2},
		{"(I)Z", false, 2},
		{"(J)V", false, 3},
		{"(JD)V", true, 4},
		{"([Ljava/lang/String;)V", true, 1},
		{"([[I)V", true, 1},
		{"([I[B)V", true, 2},
	} {
		got := countArgs(c.desc)
		if !c.static {
			got++
		}
		if got != c.want {
			t.Errorf("args of %s (static=%v): %d, want %d", c.desc, c.static, got, c.want)
		}
	}
}

func TestReadClassFullRefusesWhatTheForkDoesNotDex(t *testing.T) {
	// un lambda, o la concatenacion de strings que el JDK 9 compila como dynamic, dejan
	// invokedynamic en el pool: el lector lo dice por su nombre en vez de dexar a medias
	x := poolIndex{p: &pool{tag: map[int]byte{7: cpInvokeDynamic}}}
	if err := x.refuseOutsideSubset(); err == nil || !strings.Contains(err.Error(), "invokedynamic") {
		t.Errorf("an invokedynamic was accepted or refused in silence: %v", err)
	}
	x2 := poolIndex{p: &pool{tag: map[int]byte{3: cpMethodHandle}}}
	if err := x2.refuseOutsideSubset(); err == nil {
		t.Error("a method handle was accepted, and only a dynamic call uses one")
	}
	x3 := poolIndex{p: &pool{tag: map[int]byte{1: cpClass, 2: cpUtf8, 4: cpMethodref}}}
	if err := x3.refuseOutsideSubset(); err != nil {
		t.Errorf("the ordinary pool was refused: %v", err)
	}
}

func TestWalkJvmCodeStopsAndNamesTheByteItDoesNotKnow(t *testing.T) {
	// 0x6a es baload, fuera de los dieciseis medidos. El caminante tiene que decir cual: media
	// instruccion contada como completa es justo el casi-correcto que llega al dispositivo
	if _, err := walkJvmCode([]byte{0x2a, 0x6a, 0xb1}); err == nil {
		t.Error("an unmeasured opcode walked as if it were known")
	} else if !strings.Contains(err.Error(), "0x6a") {
		t.Errorf("the refusal does not name the byte: %v", err)
	}
	if _, err := walkJvmCode([]byte{0x12}); err == nil {
		t.Error("a truncated ldc operand was accepted")
	}
	if n, err := countJvmInstructions([]byte{0x2a, 0xb1}); err != nil || n != 2 {
		t.Errorf("two known instructions walked as %d (err %v)", n, err)
	}
}

func TestDescriptorOfKeepsWhatIsAlreadyADescriptor(t *testing.T) {
	for _, c := range []struct {
		dado, querido string
	}{
		{"com/x/Y", "Lcom/x/Y;"},
		{"Lcom/x/Y;", "Lcom/x/Y;"},
		{"[Lcom/x/Y;", "[Lcom/x/Y;"},
		{"[[I", "[[I"},
		{"I", "I"},
		{"V", "V"},
		{"", ""},
	} {
		if got := descriptorOf(c.dado); got != c.querido {
			t.Errorf("descriptorOf(%q) = %q, want %q", c.dado, got, c.querido)
		}
	}
}
