package main

// Los dos avisos que destaparon las APK de `chatbt` y `lienzo`, medidos sobre el binario 46a9a792:
// `checkSelfPermission` devolvía el cero de I sin que nadie lo dijera, e `invalidate` el cero de V.
// Aquí se comprueba que los dos tienen stub, que el permiso sigue concedido (0) y que invalidar
// no deja huella: el modelo dibuja el `onDraw` una sola vez al construir la pantalla, con una
// instancia nueva, así que programar un redibujado sería inventar un efecto que no se puede medir.
// La falsificación apaga los stubs y exige que el aviso aparezca por la vía real de `invoca`.

import (
	"strings"
	"testing"
)

// maquinaDePermisos monta la jerarquía mínima del dex para que `invoca` llegue a los stubs: la
// actividad de `chatbt` cuelga de Activity y la vista de `lienzo` cuelga de View.
func maquinaDePermisos() (*maquinaDalvik, *objetoDalvik, *objetoDalvik) {
	m := maquinaDeStubs()
	m.prog = &programaDalvik{clases: map[string]*claseDalvik{
		"Lcom/hola/chatbt/MainActivity;": {
			nombre: "Lcom/hola/chatbt/MainActivity;", super: "Landroid/app/Activity;",
		},
		"Lcom/hola/lienzo/Lienzo;": {
			nombre: "Lcom/hola/lienzo/Lienzo;", super: "Landroid/view/View;",
		},
	}}
	actividad := objDe("Lcom/hola/chatbt/MainActivity;")
	vista := objDe("Lcom/hola/lienzo/Lienzo;")
	m.actividad = actividad
	return m, actividad, vista
}

// TestStubCheckSelfPermissionConcedido comprueba el valor explícito que evita el aviso sin mover
// la rama «permiso concedido» que ya seguía la app con el cero implícito.
func TestStubCheckSelfPermissionConcedido(t *testing.T) {
	m, actividad, _ := maquinaDePermisos()
	permiso := cadenaObjeto("android.permission.BLUETOOTH_CONNECT")
	v, ok, err := m.intentaStub([]string{"Lcom/hola/chatbt/MainActivity;"},
		"checkSelfPermission", "(Ljava/lang/String;)I", []any{permiso}, actividad)
	if err != nil || !ok {
		t.Fatalf("checkSelfPermission sobre la actividad: v=%v ok=%v err=%v", v, ok, err)
	}
	if v != int32(0) {
		t.Errorf("checkSelfPermission devolvió %v y se esperaba PERMISSION_GRANTED (0)", v)
	}
	// ContextCompat se llama en estático: no hay receptor, pero la clase declarante casa.
	v, ok, err = m.intentaStub([]string{"Landroidx/core/content/ContextCompat;"},
		"checkSelfPermission", "(Ljava/lang/String;)I", []any{permiso}, nil)
	if err != nil || !ok {
		t.Fatalf("checkSelfPermission de ContextCompat: v=%v ok=%v err=%v", v, ok, err)
	}
	if v != int32(0) {
		t.Errorf("ContextCompat.checkSelfPermission devolvió %v y se esperaba 0", v)
	}
	if avisos := strings.Join(m.avisos, "\n"); strings.Contains(avisos, "no está en el dex ni en los stubs") {
		t.Errorf("checkSelfPermission cayó en el aviso de stub ausente: %s", avisos)
	}
}

// TestStubInvalidateSinHuella exige que `invalidate` se acepte sin captura, sin contador y sin
// aviso: el intérprete no tiene ciclo de dibujo y no debe fingir uno.
func TestStubInvalidateSinHuella(t *testing.T) {
	m, _, vista := maquinaDePermisos()
	candidatos := append(m.jerarquiaDelDex(vista.clase), "Lcom/hola/lienzo/Lienzo;")
	v, ok, err := m.intentaStub(candidatos, "invalidate", "()V", nil, vista)
	if err != nil || !ok {
		t.Fatalf("invalidate sobre la vista propia: v=%v ok=%v err=%v", v, ok, err)
	}
	if v != nil {
		t.Errorf("invalidate devolvió %v y se esperaba el cero de V", v)
	}
	if len(m.capturas) != 0 {
		t.Errorf("invalidate dejó %d capturas y no debe dejar ninguna: %+v", len(m.capturas), m.capturas)
	}
	if len(m.avisos) != 0 {
		t.Errorf("invalidate dejó avisos: %v", m.avisos)
	}
}

// casoPermiso es una llamada tal como llega al intérprete desde las APK reales: el descriptor del
// method_id, los registros y la clase declarante.
type casoPermiso struct {
	nombre, proto, retorno, owner string
	params                        []string
	palabras                      []int
	regs                          func(actividad, vista *objetoDalvik) []any
}

// casosPermisos reúne `checkSelfPermission` sobre la actividad e `invalidate` sobre la vista propia.
func casosPermisos() []casoPermiso {
	return []casoPermiso{
		{
			nombre: "checkSelfPermission", proto: "(Ljava/lang/String;)I", retorno: "I",
			owner: "Lcom/hola/chatbt/MainActivity;", params: []string{"Ljava/lang/String;"},
			palabras: []int{0, 1},
			regs: func(actividad, _ *objetoDalvik) []any {
				return []any{actividad, cadenaObjeto("android.permission.BLUETOOTH_CONNECT")}
			},
		},
		{
			nombre: "invalidate", proto: "()V", retorno: "V",
			owner:    "Lcom/hola/lienzo/Lienzo;",
			palabras: []int{0},
			regs: func(_, vista *objetoDalvik) []any {
				return []any{vista}
			},
		},
	}
}

// invocaCasoDePermiso mete un method_id mínimo en el dex y recorre la vía real de `invoca`.
func invocaCasoDePermiso(m *maquinaDalvik, actividad, vista *objetoDalvik, c casoPermiso) (any, error) {
	d := &dexFile{methods: []dexMethod{{
		owner: c.owner, name: c.nombre,
		proto: dexProto{params: c.params, returnType: c.retorno},
	}}}
	return m.invoca(0x6e, d, 0, c.palabras, c.regs(actividad, vista), "")
}

// TestFalsificacionAvisoDeLosDosStubs apaga los stubs y comprueba que `invoca` sí avisa y entrega
// el cero; después los enciende y exige lo contrario con la misma llamada.
func TestFalsificacionAvisoDeLosDosStubs(t *testing.T) {
	stubsFrameworkActivos = false
	defer func() { stubsFrameworkActivos = true }()
	for _, c := range casosPermisos() {
		m, actividad, vista := maquinaDePermisos()
		v, err := invocaCasoDePermiso(m, actividad, vista, c)
		if err != nil {
			t.Fatalf("%s sin stub devolvió error: %v", c.nombre, err)
		}
		if esperado := valorCero(c.retorno); v != esperado {
			t.Errorf("%s sin stub devolvió %v y se esperaba el cero %v", c.nombre, v, esperado)
		}
		avisos := strings.Join(m.avisos, "\n")
		if !strings.Contains(avisos, "no está en el dex ni en los stubs") {
			t.Errorf("%s sin stub no avisó: %v", c.nombre, m.avisos)
		}
		if !strings.Contains(avisos, c.nombre+c.proto) {
			t.Errorf("%s sin stub no nombra la firma en el aviso: %v", c.nombre, m.avisos)
		}
	}

	stubsFrameworkActivos = true
	for _, c := range casosPermisos() {
		m, actividad, vista := maquinaDePermisos()
		v, err := invocaCasoDePermiso(m, actividad, vista, c)
		if err != nil {
			t.Fatalf("%s con stub devolvió error: %v", c.nombre, err)
		}
		if esperado := valorCero(c.retorno); v != esperado {
			t.Errorf("%s con stub devolvió %v y se esperaba %v", c.nombre, v, esperado)
		}
		if avisos := strings.Join(m.avisos, "\n"); strings.Contains(avisos, "no está en el dex ni en los stubs") {
			t.Errorf("%s con stub cayó en el aviso de stub ausente: %s", c.nombre, avisos)
		}
	}
}
