package main

// Los stubs de framework que destapó la APK de ApkShare: Looper, Context, PackageManager, el builder
// de AlertDialog y el ExecutorService. Antes tenían voz en el registro de «no está en el dex ni en
// los stubs» y devolvían el cero; encadenarlos terminaba en «se llamó a
// java.util.concurrent.ExecutorService.execute sobre un objeto nulo», que es lo que estas pruebas
// falsifican con el interruptor apagado.
//
// Todas las pruebas son directas sobre `intentaStub`: no necesitan dex, jar ni compilación, y por eso
// corren en el despacho puntual sin calentar el portátil.

import (
	"strings"
	"testing"
)

// maquinaDeStubs es una máquina mínima con el mapa de avisos puesto: `avisa` lo necesita.
func maquinaDeStubs() *maquinaDalvik {
	return &maquinaDalvik{vistos: map[string]bool{}}
}

// objDe es un objeto del framework con su clase y su mapa de campos listo.
func objDe(clase string) *objetoDalvik {
	return &objetoDalvik{clase: clase, campos: map[string]any{}}
}

// TestStubsDelFrameworkDeApkShare comprueba, uno a uno, que cada método de la cadena devuelve un
// objeto no nulo (o el propio builder) y que no cae en el aviso de stub ausente.
func TestStubsDelFrameworkDeApkShare(t *testing.T) {
	m := maquinaDeStubs()
	actividad := objDe("Lshare/apps/util/MainActivity;")
	m.actividad = actividad

	casos := []struct {
		nombre     string
		candidatos []string
		proto      string
		vals       []any
		receptor   any
		clase      string
	}{
		{"getMainLooper", []string{"Landroid/os/Looper;"}, "()Landroid/os/Looper;", nil, nil,
			"Landroid/os/Looper;"},
		{"getPackageManager", []string{"Lshare/apps/util/MainActivity;"},
			"()Landroid/content/pm/PackageManager;", nil, actividad,
			"Landroid/content/pm/PackageManager;"},
		{"getApplicationContext", []string{"Lshare/apps/util/MainActivity;"},
			"()Landroid/content/Context;", nil, actividad, "Landroid/app/Application;"},
		{"newSingleThreadExecutor", []string{"Ljava/util/concurrent/Executors;"},
			"()Ljava/util/concurrent/ExecutorService;", nil, nil,
			"Ljava/util/concurrent/ExecutorService;"},
	}
	for _, c := range casos {
		v, ok, err := m.intentaStub(c.candidatos, c.nombre, c.proto, c.vals, c.receptor)
		if err != nil {
			t.Fatalf("%s devolvió error: %v", c.nombre, err)
		}
		if !ok {
			t.Fatalf("%s no lo atiende ningún stub", c.nombre)
		}
		o, esObjeto := v.(*objetoDalvik)
		if !esObjeto || o == nil {
			t.Errorf("%s devolvió %T y se esperaba un objeto no nulo", c.nombre, v)
			continue
		}
		if o.clase != c.clase {
			t.Errorf("%s devolvió %s y se esperaba %s", c.nombre, o.clase, c.clase)
		}
	}
	if avisos := strings.Join(m.avisos, "\n"); strings.Contains(avisos, "no está en el dex ni en los stubs") {
		t.Errorf("alguna llamada cayó en el aviso de stub ausente: %s", avisos)
	}
}

// TestElContextoDevueltoEncadena: `getApplicationContext()` tiene que devolver un contexto sobre el
// que `getPackageManager` también responda; si el Application no se marcara, la app recibiría el
// cero en la segunda llamada.
func TestElContextoDevueltoEncadena(t *testing.T) {
	m := maquinaDeStubs()
	actividad := objDe("Lshare/apps/util/MainActivity;")
	m.actividad = actividad
	ctx, ok, err := m.intentaStub([]string{"Lshare/apps/util/MainActivity;"},
		"getApplicationContext", "()Landroid/content/Context;", nil, actividad)
	if err != nil || !ok {
		t.Fatalf("getApplicationContext: ok=%v err=%v", ok, err)
	}
	pm, ok, err := m.intentaStub([]string{"Lshare/apps/util/MainActivity;"},
		"getPackageManager", "()Landroid/content/pm/PackageManager;", nil, ctx)
	if err != nil || !ok {
		t.Fatalf("getPackageManager sobre el Application: ok=%v err=%v", ok, err)
	}
	if pm == nil {
		t.Error("getPackageManager sobre el Application devolvió el cero")
	}
}

// TestElBuilderEncadenaComoEnElFramework: `setView` y `setCancelable` tienen que devolver el MISMO
// builder para que `create()` y `show()` sigan la cadena, y `hide()` quita el diálogo de la capa.
func TestElBuilderEncadenaComoEnElFramework(t *testing.T) {
	m := maquinaDeStubs()
	builder := objDe("Landroid/app/AlertDialog$Builder;")
	vista := objDe("Landroid/view/View;")
	for _, c := range []struct {
		nombre string
		proto  string
		vals   []any
	}{
		{"setView", "(Landroid/view/View;)Landroid/app/AlertDialog$Builder;", []any{vista}},
		{"setCancelable", "(Z)Landroid/app/AlertDialog$Builder;", []any{int32(0)}},
	} {
		v, ok, err := m.intentaStub([]string{"Landroid/app/AlertDialog$Builder;"},
			c.nombre, c.proto, c.vals, builder)
		if err != nil || !ok {
			t.Fatalf("%s: ok=%v err=%v", c.nombre, ok, err)
		}
		if v != builder {
			t.Errorf("%s no devolvió el builder: %v", c.nombre, v)
		}
	}
	d := creaDialogo(builder)
	m.apilaDialogo(d)
	if !m.dialogoAbierto(d) {
		t.Fatal("el diálogo creado no estaba abierto")
	}
	if _, ok, err := m.intentaStub([]string{"Landroid/app/Dialog;"}, "hide", "()V", nil, d); err != nil || !ok {
		t.Fatalf("hide: ok=%v err=%v", ok, err)
	}
	if m.dialogoAbierto(d) {
		t.Error("hide no quitó el diálogo de la capa")
	}
}

// TestElEjecutorEncolaElRunnable: `execute` tiene que encolar el Runnable en la cola lógica, como un
// `post`, en vez de devolver el cero y reventar al llamarlo.
func TestElEjecutorEncolaElRunnable(t *testing.T) {
	m := maquinaDeStubs()
	ejecutor := objDe("Ljava/util/concurrent/ExecutorService;")
	runnable := objDe("Lshare/apps/util/MainActivity$1;")
	v, ok, err := m.intentaStub([]string{"Ljava/util/concurrent/ExecutorService;"},
		"execute", "(Ljava/lang/Runnable;)V", []any{runnable}, ejecutor)
	if err != nil || !ok {
		t.Fatalf("execute: ok=%v err=%v", ok, err)
	}
	if v != nil {
		t.Errorf("execute devolvió %v y es un método void", v)
	}
	if len(m.cola) != 1 {
		t.Fatalf("execute encoló %d tareas y se esperaba 1", len(m.cola))
	}
	if m.cola[0].runnable != runnable {
		t.Error("execute encoló un Runnable distinto del que recibió")
	}
}

// TestLaListaDePaquetesEsVaciaPeroNoNula: sin sistema de paquetes, `getInstalledPackages` devuelve
// una lista vacía y su `iterator()` un iterador sin elementos, para que el bucle de la app termine
// sin un receptor nulo.
func TestLaListaDePaquetesEsVaciaPeroNoNula(t *testing.T) {
	m := maquinaDeStubs()
	pm := objDe("Landroid/content/pm/PackageManager;")
	lista, ok, err := m.intentaStub([]string{"Landroid/content/pm/PackageManager;"},
		"getInstalledPackages", "(I)Ljava/util/List;", []any{int32(0)}, pm)
	if err != nil || !ok || lista == nil {
		t.Fatalf("getInstalledPackages: v=%v ok=%v err=%v", lista, ok, err)
	}
	it, ok, err := m.intentaStub([]string{"Ljava/util/List;"}, "iterator", "()Ljava/util/Iterator;",
		nil, lista)
	if err != nil || !ok || it == nil {
		t.Fatalf("iterator: v=%v ok=%v err=%v", it, ok, err)
	}
	v, ok, err := m.intentaStub([]string{"Ljava/util/Iterator;"}, "hasNext", "()Z", nil, it)
	if err != nil || !ok {
		t.Fatalf("hasNext: ok=%v err=%v", ok, err)
	}
	if v != int32(0) {
		t.Errorf("hasNext devolvió %v y la lista está vacía", v)
	}
}

// TestFalsificacionConLosStubsApagados reproduce el fallo original: sin los stubs,
// `newSingleThreadExecutor` no se resuelve y el intérprete entrega el cero (nil); la llamada
// siguiente a `execute` sobre ese cero no casa con ningún stub y es el «se llamó a
// java.util.concurrent.ExecutorService.execute sobre un objeto nulo» del registro de la APK.
func TestFalsificacionConLosStubsApagados(t *testing.T) {
	stubsFrameworkActivos = false
	defer func() { stubsFrameworkActivos = true }()

	m := maquinaDeStubs()
	v, ok, err := m.intentaStub([]string{"Ljava/util/concurrent/Executors;"},
		"newSingleThreadExecutor", "()Ljava/util/concurrent/ExecutorService;", nil, nil)
	if err != nil || ok {
		t.Fatalf("con los stubs apagados newSingleThreadExecutor se resolvió: v=%v ok=%v err=%v",
			v, ok, err)
	}
	ejecutor := valorCero("Ljava/util/concurrent/ExecutorService;")
	if ejecutor != nil {
		t.Fatalf("el cero del tipo de retorno es %v y se esperaba el receptor nulo", ejecutor)
	}
	v, ok, err = m.intentaStub([]string{"Ljava/util/concurrent/ExecutorService;"},
		"execute", "(Ljava/lang/Runnable;)V", []any{objDe("Ljava/lang/Runnable;")}, ejecutor)
	if err != nil || ok {
		t.Fatalf("execute sobre el cero se resolvió: v=%v ok=%v err=%v", v, ok, err)
	}
}
