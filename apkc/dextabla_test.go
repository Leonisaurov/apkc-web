package main

import (
	"strings"
	"testing"
)

// tablasDeLasSeis interning de las seis clases del fixture, con el orden ya aplicado.
func tablasDeLasSeis(t *testing.T) *dexTables {
	t.Helper()
	clases := leeLasSeis(t)
	tab := newDexTables()
	if err := tab.collectClasses(clases); err != nil {
		t.Fatalf("collecting the classes: %v", err)
	}
	tab.orderTables()
	return tab
}

// TestDexTablesMatchWhatD8Interned es la comprobacion de 2G-D: las tablas de indices que este
// codigo deriva de los .class tienen que salir con los mismos elementos y en el mismo orden que
// salieron del d8 real. La coleccion se compara entera, no como muestra, porque un item de mas o
// de menos desplaza todos los indices posteriores y eso es exactamente el defecto que un apk
// instalaria en vez de denunciar.
func TestDexTablesMatchWhatD8Interned(t *testing.T) {
	d, err := readDex(fixtureDex(t))
	if err != nil {
		t.Fatal(err)
	}
	tab := tablasDeLasSeis(t)

	if solo := diferencias(d.types, tab.types); len(solo) > 0 {
		t.Errorf("types that only d8 has: %v", solo)
	}
	if solo := diferencias(tab.types, d.types); len(solo) > 0 {
		t.Errorf("types only we interned: %v", solo)
	}
	if !iguales(d.types, tab.types) {
		t.Errorf("type_ids in a different order:\n  d8       %v\n  nosotros %v", d.types, tab.types)
	}

	if len(d.protos) != len(tab.protos) {
		t.Errorf("proto_ids: d8 wrote %d, we interned %d", len(d.protos), len(tab.protos))
	}
	if !iguales(clavesProtosDe(d.protos), clavesProtos(tab.protos)) {
		t.Errorf("proto_ids distinto:\n  d8       %v\n  nosotros %v",
			clavesProtosDe(d.protos), clavesProtos(tab.protos))
	}

	if len(d.fields) != len(tab.fields) {
		t.Errorf("field_ids: d8 wrote %d, we interned %d", len(d.fields), len(tab.fields))
	}
	if !iguales(clavesCamposDe(d.fields), clavesCampos(tab.fields)) {
		t.Errorf("field_ids distinto:\n  d8       %v\n  nosotros %v", clavesCamposDe(d.fields), clavesCampos(tab.fields))
	}

	if len(d.methods) != len(tab.methods) {
		t.Errorf("method_ids: d8 wrote %d, we interned %d", len(d.methods), len(tab.methods))
	}
	if !iguales(clavesMetodosDe(d.methods), clavesMetodos(tab.methods)) {
		t.Errorf("method_ids distinto:\n  d8       %v\n  nosotros %v",
			clavesMetodosDe(d.methods), clavesMetodos(tab.methods))
	}
}

// TestDexTablesShortiesAreTheMeasuredLetters: los shorties no se inventan. Los seis que se ven en
// la tabla de protos de d8 estan en docs/evidencia.md §13; "Z" no es un shorty ahi, es el
// descriptor del tipo booleano que devuelven onMenuItemClick y on onCreateOptionsMenu, y por eso se
// comprueba como cadena, no como shorty.
func TestDexTablesShortiesAreTheMeasuredLetters(t *testing.T) {
	esperados := map[string]bool{"V": false, "VI": false, "VL": false, "ZL": false, "LL": false, "LLLI": false}
	for _, p := range tablasDeLasSeis(t).protos {
		if _, ok := esperados[p.shorty]; ok {
			esperados[p.shorty] = true
		}
	}
	for s, visto := range esperados {
		if !visto {
			t.Errorf("no proto produced the shorty %q, which d8's proto table has", s)
		}
	}
	esta := false
	for _, s := range tablasDeLasSeis(t).strings {
		if s == "Z" {
			esta = true
		}
	}
	if !esta {
		t.Error(`"Z" is a type descriptor d8 interns (the boolean the two listeners return) and it is missing`)
	}
}

func TestSplitDescriptorMatchesTheMeasuredShorties(t *testing.T) {
	for _, c := range []struct {
		desc, shorty, ret string
		params            []string
	}{
		{"()V", "V", "V", nil},
		{"(Landroid/os/Bundle;)V", "VL", "V", []string{"Landroid/os/Bundle;"}},
		{"(Landroid/view/Menu;)Z", "ZL", "Z", []string{"Landroid/view/Menu;"}},
		{"(I)V", "VI", "V", []string{"I"}},
		{"([Ljava/lang/String;)V", "VL", "V", []string{"[Ljava/lang/String;"}},
		{"(J)D", "DJ", "D", []string{"J"}},
		{"(Landroid/content/Context;Ljava/lang/CharSequence;I)Landroid/widget/Toast;", "LLLI",
			"Landroid/widget/Toast;",
			[]string{"Landroid/content/Context;", "Ljava/lang/CharSequence;", "I"}},
	} {
		shorty, ret, params := splitDescriptor(c.desc)
		if shorty != c.shorty || ret != c.ret {
			t.Errorf("%s: shorty=%q ret=%q, want %q and %q", c.desc, shorty, ret, c.shorty, c.ret)
		}
		if len(params) != len(c.params) {
			t.Errorf("%s: %d params %v, want %d", c.desc, len(params), params, len(c.params))
			continue
		}
		for i := range params {
			if params[i] != c.params[i] {
				t.Errorf("%s: param %d is %q, want %q", c.desc, i, params[i], c.params[i])
			}
		}
	}
}

// TestDexTablesAreDeterministic: the same classes, interned twice, must come out in the same order.
// An order that depends on how a Go map happened to iterate produces a different file on every
// run, and "the same file" is the property an incremental build sells.
func TestDexTablesAreDeterministic(t *testing.T) {
	a, b := tablasDeLasSeis(t), tablasDeLasSeis(t)
	if !iguales(a.strings, b.strings) || !iguales(a.types, b.types) {
		t.Error("two passes over the same classes gave different orders")
	}
	if len(a.methods) != len(b.methods) || len(a.fields) != len(b.fields) {
		t.Fatal("two passes interned different numbers of members")
	}
	for i := range a.methods {
		if a.methods[i] != b.methods[i] {
			t.Fatalf("method #%d: %v against %v", i, a.methods[i], b.methods[i])
		}
	}
}

// TestDexTablesFollowTheCodeNotTheDeclaration: the anonymous class calls Toast.makeText and
// Toast.show, and onCreate references R$layout and R$id members. Those arrive through the operands
// of instructions, so if the walker of the code were skipped the tables would come out short.
func TestDexTablesFollowTheCodeNotTheDeclaration(t *testing.T) {
	tab := tablasDeLasSeis(t)
	para := func(busca string) bool {
		for _, m := range tab.methods {
			if m.owner == busca {
				return true
			}
		}
		return false
	}
	if !para("Landroid/widget/Toast;") {
		t.Error("Toast is only reached from the code's operands; it is missing, so the code was not followed")
	}
	if !para("Landroid/app/Activity;") {
		t.Error("the invokespecial to Activity.<init> never made it into the method table")
	}
	for _, s := range []string{"Clicked!", "Hello World!", "MainActivity.java", "pulsar"} {
		esta := false
		for _, x := range tab.strings {
			if x == s {
				esta = true
			}
		}
		if !esta {
			t.Errorf("%q should have been interned and was not", s)
		}
	}
	if !iguales(tab.strings, ordenadoPorUnidad(tab.strings)) {
		t.Error("the string table is not in code-unit order after orderTables")
	}
}

func ordenadoPorUnidad(s []string) []string {
	out := append([]string{}, s...)
	sortByMUTF8(out)
	return out
}

func diferencias(a, b []string) []string {
	enB := map[string]bool{}
	for _, x := range b {
		enB[x] = true
	}
	fuera := []string{}
	for _, x := range a {
		if !enB[x] {
			fuera = append(fuera, x)
		}
	}
	return fuera
}

func iguales(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func clavesProtos(ps []protoKey) []string {
	out := []string{}
	for _, p := range ps {
		out = append(out, p.shorty+"|"+p.ret+"|("+p.params+")")
	}
	return out
}

func clavesProtosDe(ps []dexProto) []string {
	out := []string{}
	for _, p := range ps {
		out = append(out, p.shorty+"|"+p.returnType+"|("+strings.Join(p.params, "")+")")
	}
	return out
}

func clavesCampos(k []fieldKey) []string {
	out := []string{}
	for _, f := range k {
		out = append(out, f.owner+"->"+f.name+":"+f.typeOf)
	}
	return out
}

func clavesCamposDe(fs []dexField) []string {
	out := []string{}
	for _, f := range fs {
		out = append(out, f.owner+"->"+f.name+":"+f.typeOf)
	}
	return out
}

func clavesMetodos(k []methodKey) []string {
	out := []string{}
	for _, m := range k {
		shorty, ret, params := splitDescriptor(m.desc)
		out = append(out, m.owner+"->"+m.name+"("+strings.Join(params, "")+")"+ret+"#"+shorty)
	}
	return out
}

func clavesMetodosDe(ms []dexMethod) []string {
	out := []string{}
	for _, m := range ms {
		out = append(out, m.owner+"->"+m.name+"("+strings.Join(m.proto.params, "")+")"+m.proto.returnType+"#"+m.proto.shorty)
	}
	return out
}
