package main

// Pruebas de las fases A1 y A2 del hot swap sobre estado vivo:
//   - A1: el códec explícito conserva la identidad de puntero de los oyentes y sus vistas, incluso
//     después de pasar por JSON, y rehidrata cada `code_item` desde el dex que le corresponde;
//   - A2: el delta distingue alta/cambio/baja, la baja limpia las tres listas de oyentes y el anclaje
//     es por id de vista, no por el ordinal `$N` del anónimo de javac.
//
// La renumeración REAL que hace javac al borrar un anónimo intermedio no se mide aquí: el delta se
// prueba con firmas armadas a mano para no dar por cerrado un comportamiento del compilador que no
// hemos falsificado. Lo que se falsifica es el mecanismo de anclaje, no javac.

import (
	"encoding/json"
	"fmt"
	"testing"
)

// maquinaConCincoOyentes arma una sesión con cinco vistas y cinco oyentes registrados. El oyente 3
// guarda en sus campos una referencia al oyente 1 y la actividad es el oyente 1: dos aliás que el
// códec tiene que conservar como el MISMO puntero, no como dos copias con el mismo contenido.
func maquinaConCincoOyentes(t *testing.T) (*maquinaDalvik, []*objetoDalvik, []*objetoDalvik) {
	t.Helper()
	d, err := readDex(dexDeFuentes(t, fuenteJava{nombre: "Saludo.java", contenido: fuenteSaludoV1}))
	if err != nil {
		t.Fatalf("el dex de la sesión no se lee: %v", err)
	}
	prog, err := cargaPrograma(d)
	if err != nil {
		t.Fatalf("el programa no carga: %v", err)
	}
	m := nuevaMaquina(prog, nil)

	vistas := make([]*objetoDalvik, 5)
	oyentes := make([]*objetoDalvik, 5)
	for i := 0; i < 5; i++ {
		vistas[i] = &objetoDalvik{
			clase: "Landroid/view/View;", id: uint32(100 + i),
			campos: map[string]any{}, vistas: map[uint32]*objetoDalvik{},
		}
		oyentes[i] = &objetoDalvik{
			clase:  fmt.Sprintf("Lcom/hola/MainActivity$%d;", i+1),
			campos: map[string]any{}, vistas: map[uint32]*objetoDalvik{},
		}
		vistas[i].oyente = oyentes[i]
		m.vistas[vistas[i].id] = vistas[i]
		m.oyentes = append(m.oyentes, oyenteRegistrado{oyente: oyentes[i], vista: vistas[i]})
	}
	oyentes[2].campos["Lcom/hola/Oyente;.comun"] = oyentes[0]
	m.actividad = oyentes[0]
	return m, vistas, oyentes
}

// TestRoundTripCincoOyentesConservaIdentidad es la falsificación de A1: captura, pasa por JSON y
// rehidrata. Comprueba PUNTEROS, no contenidos: la vista de cada registro es la misma instancia que
// la del mapa de vistas, el campo `oyente` de esa vista es el mismo oyente del registro, los aliás
// siguen compartidos y cada método recupera su `code_item` del dex correcto.
func TestRoundTripCincoOyentesConservaIdentidad(t *testing.T) {
	m, vistas, oyentes := maquinaConCincoOyentes(t)

	estado := capturaEstado(m)
	bytes, err := json.Marshal(estado)
	if err != nil {
		t.Fatalf("el estado no se serializa: %v", err)
	}
	var leido estadoWasm
	if err := json.Unmarshal(bytes, &leido); err != nil {
		t.Fatalf("el estado no se relee: %v", err)
	}
	salida, err := restauraEstado(&leido)
	if err != nil {
		t.Fatalf("el estado no se rehidrata: %v", err)
	}

	if len(salida.oyentes) != 5 {
		t.Fatalf("la sesión rehidratada tiene %d oyentes y se esperaban 5", len(salida.oyentes))
	}
	for i, reg := range salida.oyentes {
		o, ok := reg.oyente.(*objetoDalvik)
		if !ok || o == nil {
			t.Fatalf("el oyente %d no es un objeto rehidratado: %T", i, reg.oyente)
		}
		v, ok := reg.vista.(*objetoDalvik)
		if !ok || v == nil {
			t.Fatalf("la vista %d no es un objeto rehidratado: %T", i, reg.vista)
		}
		// la vista del registro y la del mapa son el MISMO puntero
		if salida.vistas[v.id] != v {
			t.Fatalf("el oyente %d apunta a una vista que no es la del mapa de vistas", i)
		}
		// y el campo `oyente` de esa vista es el MISMO oyente del registro
		if v.oyente != o {
			t.Fatalf("la vista %d tiene un oyente distinto del registro (punteros %p vs %p)",
				v.id, v.oyente, o)
		}
		if v.id != vistas[i].id || o.clase != oyentes[i].clase {
			t.Fatalf("el registro %d perdió su identidad: vista %d/%q, esperado %d/%q",
				i, v.id, o.clase, vistas[i].id, oyentes[i].clase)
		}
		// los objetos son nuevos: no estamos comparando contra el original por accidente
		if o == oyentes[i] || v == vistas[i] {
			t.Fatalf("el registro %d reutilizó el objeto original en vez de rehidratarlo", i)
		}
	}

	// los aliás del grafo siguen siendo el mismo puntero
	comun := salida.oyentes[2].oyente.(*objetoDalvik).campos["Lcom/hola/Oyente;.comun"]
	if comun != salida.oyentes[0].oyente {
		t.Fatalf("el campo compartido no apunta al mismo oyente rehidratado: %p vs %p",
			comun, salida.oyentes[0].oyente)
	}
	if salida.actividad != salida.oyentes[0].oyente {
		t.Fatal("la actividad rehidratada no es el mismo puntero que el primer oyente")
	}

	// el `code_item` se rehidrata por id de dex: no se serializaron sus índices
	antes := m.prog.clases["Lcom/hola/Saludo;"].metodos["valor()I"]
	despues := salida.prog.clases["Lcom/hola/Saludo;"].metodos["valor()I"]
	if antes == nil || antes.item == nil || despues == nil || despues.item == nil {
		t.Fatalf("el método valor()I no tiene cuerpo antes o después del round-trip: %v / %v",
			antes, despues)
	}
	if despues.dex == nil {
		t.Fatal("el método rehidratado perdió su dex")
	}
	if len(despues.item.insns) != len(antes.item.insns) {
		t.Fatalf("el code_item rehidratado tiene %d palabras y el original %d",
			len(despues.item.insns), len(antes.item.insns))
	}
	for i := range antes.item.insns {
		if despues.item.insns[i] != antes.item.insns[i] {
			t.Fatalf("la palabra %d del code_item cambió al rehidratar", i)
		}
	}
}

// TestRoundTripEstaticoConObjetoConservaPuntero falsifica el defecto que señaló la crítica: un
// `putstatic` que guarda una vista en un campo estático tiene que rehidratarse como el MISMO puntero
// que esa vista en el mapa, no como el objeto 0 de la tabla. El escalar de al lado comprueba que el
// arreglo no se rompió al añadir las referencias de los estáticos como raíces del grafo.
func TestRoundTripEstaticoConObjetoConservaPuntero(t *testing.T) {
	m, vistas, _ := maquinaConCincoOyentes(t)
	const clase = "Lcom/hola/Saludo;"
	const claveUltimo = "Lcom/hola/Saludo;.ultimo"
	const claveBase = "Lcom/hola/Saludo;.base"
	cl := m.prog.clases[clase]
	cl.estaticos[claveUltimo] = vistas[3]
	cl.estaticos[claveBase] = int32(40)

	bytes, err := json.Marshal(capturaEstado(m))
	if err != nil {
		t.Fatalf("el estado no se serializa: %v", err)
	}
	var leido estadoWasm
	if err := json.Unmarshal(bytes, &leido); err != nil {
		t.Fatalf("el estado no se relee: %v", err)
	}
	salida, err := restauraEstado(&leido)
	if err != nil {
		t.Fatalf("el estado no se rehidrata: %v", err)
	}

	estatico, ok := salida.prog.clases[clase].estaticos[claveUltimo].(*objetoDalvik)
	if !ok || estatico == nil {
		t.Fatalf("el estático no rehidrató como objeto: %T",
			salida.prog.clases[clase].estaticos[claveUltimo])
	}
	if estatico != salida.vistas[vistas[3].id] {
		t.Fatalf("el estático apunta a %p y la vista del mapa a %p: no es el mismo puntero",
			estatico, salida.vistas[vistas[3].id])
	}
	if estatico == vistas[3] {
		t.Fatal("el estático reutilizó el objeto original en vez de rehidratarlo")
	}
	if got := salida.prog.clases[clase].estaticos[claveBase]; got != int32(40) {
		t.Fatalf("el estático escalar valía %v (%T) y se esperaba int32(40)", got, got)
	}
}

// TestDeltaBorraElOyenteDosSinRenumerar falsifica el caso de renumeración EN EL PLANO DE LOS OYENTES:
// el delta, que decide por nombre de clase con `$N`, reporta la baja de `$3` (el anónimo que ya no
// está en el dex nuevo), pero el oyente que desapareció de la UI es el de la vista 101, vivo en la
// sesión como `$2`. Anclando por id de vista se retira ese y sobrevive el de la vista 102, aunque su
// clase `$3` sea justo la que nombra la baja: con anclaje por clase se habría borrado el equivocado.
// Las firmas se arman a mano; la renumeración REAL de javac sigue sin medirse y no se da por cerrada.
func TestDeltaBorraElOyenteDosSinRenumerar(t *testing.T) {
	m, vistas, oyentes := maquinaConCincoOyentes(t)
	// solo tres vistas participan en este escenario
	m.oyentes = m.oyentes[:3]
	for i := 3; i < len(vistas); i++ {
		delete(m.vistas, vistas[i].id)
	}
	const proto = "(Landroid/view/View;)V"
	firma := func(clase string, hash uint64) firmaMetodo {
		return firmaMetodo{Clase: clase, Nombre: "onClick", Proto: proto, Hash: hash}
	}
	viejas := []firmaMetodo{
		firma("Lcom/hola/MainActivity$1;", 11),
		firma("Lcom/hola/MainActivity$2;", 22),
		firma("Lcom/hola/MainActivity$3;", 33),
	}
	// al borrar el anónimo intermedio, el antiguo $3 pasa a llamarse $2 en el dex nuevo
	nuevas := []firmaMetodo{
		firma("Lcom/hola/MainActivity$1;", 11),
		firma("Lcom/hola/MainActivity$2;", 33),
	}
	cambios := deltaDeFirmas(viejas, nuevas)
	var baja *cambioDex
	for i := range cambios {
		if cambios[i].Tipo == "baja" {
			baja = &cambios[i]
		}
	}
	if baja == nil || baja.Clase != "Lcom/hola/MainActivity$3;" {
		t.Fatalf("la baja no es la del anónimo $3 que renombró javac: %+v", cambios)
	}
	// el ancla es el id de la vista, no el `$3` del delta ni el `$2` que tiene el oyente vivo
	baja.Vista = vistas[1].id

	cl := m.prog.clases["Lcom/hola/MainActivity$2;"]
	if cl == nil {
		cl = &claseDalvik{nombre: "Lcom/hola/MainActivity$2;", metodos: map[string]*metodoDalvik{}}
		m.prog.clases[cl.nombre] = cl
	}
	cl.metodos["onClick"+proto] = &metodoDalvik{nombre: "onClick", proto: proto}

	if _, err := m.aplicaDelta([]cambioDex{*baja}, nil); err != nil {
		t.Fatalf("aplicar la baja falló: %v", err)
	}
	if _, hay := cl.metodos["onClick"+proto]; hay {
		t.Fatal("la baja no retiró el método de la clase que la vista tiene de verdad")
	}

	if len(m.oyentes) != 2 {
		t.Fatalf("quedan %d oyentes y se esperaban 2: %+v", len(m.oyentes), m.oyentes)
	}
	porId := map[uint32]*objetoDalvik{}
	for _, reg := range m.oyentes {
		v := reg.vista.(*objetoDalvik)
		porId[v.id] = reg.oyente.(*objetoDalvik)
	}
	if porId[vistas[0].id] != oyentes[0] || porId[vistas[2].id] != oyentes[2] {
		t.Fatalf("la baja reindexó o borró el oyente equivocado: %+v", porId)
	}
	if porId[vistas[2].id].clase != "Lcom/hola/MainActivity$3;" {
		t.Fatalf("sobrevivió el oyente equivocado: su clase es %q", porId[vistas[2].id].clase)
	}
	if _, hay := porId[vistas[1].id]; hay {
		t.Fatal("el oyente de la vista intermedia sigue registrado")
	}
	if vistas[1].oyente != nil {
		t.Fatalf("la vista intermedia conserva su campo oyente: %p", vistas[1].oyente)
	}
	if vistas[0].oyente != oyentes[0] || vistas[2].oyente != oyentes[2] {
		t.Fatal("la baja tocó el campo oyente de las vistas que sobreviven")
	}
}

// TestElDeltaEsAtomicoCuandoFallaUnAlta falsifica la atomicidad que reclamó la crítica: una baja
// seguida de un alta que el dex nuevo no trae no debe retirar ni el método ni los oyentes. Se comparan
// PUNTEROS delante y detrás para que un «recrear igual» no pase por conservación de identidad.
func TestElDeltaEsAtomicoCuandoFallaUnAlta(t *testing.T) {
	m, _, _, _ := sesionDelHotSwap(t)
	const clase = "Lcom/hola/Saludo;"
	const proto = "()V"
	cl := m.prog.clases[clase]
	antes := cl.metodos["sube"+proto]
	if antes == nil {
		t.Fatal("la prueba necesita que sube()V esté en la tabla de la sesión")
	}
	numAntes := len(cl.metodos)

	vista := &objetoDalvik{clase: "Landroid/view/View;", id: 7, campos: map[string]any{}}
	oyente := &objetoDalvik{clase: clase, campos: map[string]any{}}
	vista.oyente = oyente
	m.oyentes = []oyenteRegistrado{{oyente: oyente, vista: vista}}

	d2, err := readDex(dexDeFuentes(t, fuenteJava{nombre: "Saludo.java", contenido: fuenteSaludoV2}))
	if err != nil {
		t.Fatalf("el dex nuevo no se lee: %v", err)
	}
	cambios := []cambioDex{
		{Tipo: "baja", Clase: clase, Nombre: "sube", Proto: proto},
		{Tipo: "alta", Clase: clase, Nombre: "noExiste", Proto: "()I"},
	}
	if _, err := m.aplicaDelta(cambios, d2); err == nil {
		t.Fatal("el delta aceptó un alta que el dex nuevo no trae")
	}

	if len(cl.metodos) != numAntes || cl.metodos["sube"+proto] != antes {
		t.Fatalf("la baja se aplicó aunque falló el alta: tabla de %d a %d métodos, sube=%p",
			numAntes, len(cl.metodos), cl.metodos["sube"+proto])
	}
	if _, hay := cl.metodos["noExiste()I"]; hay {
		t.Fatal("el alta fallida dejó un método nuevo en la tabla")
	}
	if len(m.oyentes) != 1 || m.oyentes[0].oyente != oyente || m.oyentes[0].vista != vista {
		t.Fatalf("la baja se aplicó aunque falló el alta: oyentes=%+v", m.oyentes)
	}
	if vista.oyente != oyente {
		t.Fatal("la baja borró el campo oyente de la vista aunque falló el alta")
	}
}

// TestRetiraOyentesDeClaseLimpiaTresSitios comprueba las tres listas que deja `setOnClickListener`:
// el registro de la máquina, el campo `oyente` de la vista y el registro de `OnItemClickListener`.
// Un oyente de otra clase en cada lista debe sobrevivir intacto.
func TestRetiraOyentesDeClaseLimpiaTresSitios(t *testing.T) {
	const borrar = "Lcom/hola/Borrar;"
	const sobrevivir = "Lcom/hola/Sobrevivir;"

	crea := func(clase string, id uint32) (*objetoDalvik, *objetoDalvik) {
		vista := &objetoDalvik{clase: "Landroid/view/View;", id: id, campos: map[string]any{}}
		oyente := &objetoDalvik{clase: clase, campos: map[string]any{}}
		vista.oyente = oyente
		return vista, oyente
	}
	vBorrar, oBorrar := crea(borrar, 1)
	vSobrevive, oSobrevive := crea(sobrevivir, 2)
	listaBorrar, oListaBorrar := crea(borrar, 3)
	listaSobrevive, oListaSobrevive := crea(sobrevivir, 4)

	m := nuevaMaquina(nil, nil)
	m.oyentes = []oyenteRegistrado{
		{oyente: oBorrar, vista: vBorrar},
		{oyente: oSobrevive, vista: vSobrevive},
	}
	m.oyentesItem = []oyenteItemRegistrado{
		{oyente: oListaBorrar, vista: listaBorrar},
		{oyente: oListaSobrevive, vista: listaSobrevive},
	}

	m.retiraOyentesDeClase(borrar, 0)

	if len(m.oyentes) != 1 || m.oyentes[0].oyente != oSobrevive || m.oyentes[0].vista != vSobrevive {
		t.Fatalf("m.oyentes quedó mal tras la limpieza: %+v", m.oyentes)
	}
	if vBorrar.oyente != nil {
		t.Fatalf("el campo oyente de la vista borrada no se limpió: %p", vBorrar.oyente)
	}
	if vSobrevive.oyente != oSobrevive {
		t.Fatal("se limpió el campo oyente de la vista que debía sobrevivir")
	}
	if len(m.oyentesItem) != 1 || m.oyentesItem[0].oyente != oListaSobrevive ||
		m.oyentesItem[0].vista != listaSobrevive {
		t.Fatalf("m.oyentesItem quedó mal tras la limpieza: %+v", m.oyentesItem)
	}
	if listaBorrar.oyente != oListaBorrar {
		t.Fatal("se tocó el campo oyente de una lista que la limpieza no debía recorrer")
	}
}

// TestElDeltaNoConfundeCambioConAltaNiBaja cierra la tercera pata del delta: un mismo método con
// distinto hash es un cambio (no un alta más una baja), y un método nuevo es un alta.
func TestElDeltaNoConfundeCambioConAltaNiBaja(t *testing.T) {
	const proto = "()I"
	f := func(nombre string, hash uint64) firmaMetodo {
		return firmaMetodo{Clase: "Lcom/hola/Saludo;", Nombre: nombre, Proto: proto, Hash: hash}
	}
	cambios := deltaDeFirmas(
		[]firmaMetodo{f("valor", 1), f("viejo", 9)},
		[]firmaMetodo{f("valor", 2), f("nuevo", 3)},
	)
	porClave := map[string]string{}
	for _, c := range cambios {
		porClave[c.Nombre] = c.Tipo
	}
	if porClave["valor"] != "cambio" || porClave["viejo"] != "baja" || porClave["nuevo"] != "alta" {
		t.Fatalf("el delta clasificó mal: %+v", porClave)
	}
	if len(cambios) != 3 {
		t.Fatalf("el delta dio %d cambios y se esperaban 3: %+v", len(cambios), cambios)
	}
}
