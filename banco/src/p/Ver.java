package p;

// Un static de clase, una cuenta entera, un `if` con la condición invertida y un `System.out.println`
// con concatenación: lo que este fork emite y dexa hoy sin rechazar nada. Medido en el parrafo 61:
// nuestro emisor escribe `if_icmple 24` (al retorno), igual que javac, y `d8` lo baja a `if-le v1, v2`
// con el brazo `then` cayendo dentro del `if`. El `println` es el que hizo falta para que el banco
// compare **texto** además del código de salida, y el que destapó la elección de sobrecarga del §62.
public class Ver {
    static int total = 5;

    public static void cuenta() {
        total = total + 2;
        if (total > 10) {
            total = total * 3;
        }
        System.out.println("total " + total);
    }

    public static void suma(int n) {
        int i = 0;
        while (i < n) {
            total = total + i;
            i++;
        }
    }

    public static void elige(int n) {
        switch (n) {
            case 0: total = total + 10; break;
            case 1: total = total + 20; break;
            default: total = total + 1;
        }
    }

    // Arreglos: el alta, `.length`, el indice que se lee y se escribe, y la cuenta que pisa un
    // operando vivo (§66). `sumaArreglo(4)` es 10.
    public static int sumaArreglo(int n) {
        int[] v = new int[n];
        int i = 0;
        while (i < v.length) {
            v[i] = i + 1;
            i++;
        }
        int s = 0;
        for (int j = 0; j < v.length; j++) {
            s = s + v[j];
        }
        return s;
    }

    // Un `do-while` (§69): el cuerpo corre al menos una vez y la condición vuelve a la cabecera.
    public static int alMenosUna(int n) {
        int k = 0;
        do {
            k++;
        } while (k < n);
        return k;
    }

    // `break` y `continue` (§70): el primero sale del bucle, el segundo salta al paso del `for`.
    public static int primerIgual(int n) {
        int i = 0;
        while (i < n) {
            if (i == 2) { break; }
            i++;
        }
        return i;
    }

    public static int sinDos(int n) {
        int s = 0;
        for (int i = 0; i < n; i++) {
            if (i == 2) { continue; }
            s = s + i;
        }
        return s;
    }

    // Un `try/catch` (§71): el índice fuera de rango se atrapa y devuelve -1.
    public static int leeSeguro(int[] v, int i) {
        try {
            return v[i];
        } catch (ArrayIndexOutOfBoundsException e) {
            return -1;
        }
    }

    // Miembros privados y una anónima que los toca (§72): el campo y los métodos privados del exterior
    // salen de javac como `access$NNN`, y el dex los llama con invoke-direct/invoke-static. El valor que
    // se devuelve (10) es el que comprueba que los cinco caminos hicieron lo mismo.
    private int pulsos = 0;
    private static int oculto = 0;

    private void sube() { pulsos = pulsos + 1; }

    private static void guarda() { oculto = oculto + 2; }

    private int lee() { return pulsos; }

    public int privados() {
        Runnable r = new Runnable() {
            public void run() {
                pulsos = pulsos + 3;
                sube();
                guarda();
                oculto = oculto + lee();
            }
        };
        r.run();
        return pulsos + oculto;
    }

    // Un inicializador de arreglo (§73): `new int[] {…}` es el largo, el `newarray` y por cada
    // elemento `dup`, indice, valor y `iastore` —la forma de javac—. Devuelve 6.
    public static int inicial() {
        int[] v = new int[] {1, 2, 3};
        int s = 0;
        for (int i = 0; i < v.length; i++) {
            s = s + v[i];
        }
        return s;
    }

    // Un `long` de punta a punta (§74): campo estático ancho, una cuenta con dos argumentos largos y
    // un local ancho que se reasigna. Son 8 y 306.
    static long saldo = 5L;

    public static long cuentaLarga(long a, long b) { return a + b * 2L; }

    public static long acumula() {
        long x = 1L;
        x = x + 300L;
        saldo = saldo + x;
        return saldo;
    }

    // El `finally` con dos `catch` en el mismo `try` (§75): javac copia el bloque al pie de cada
    // camino y el ultimo manejador es el `any`, que guarda la excepcion y la re-lanza. Devuelve 22
    // por el camino normal y 7 por el que atrapa la division por cero.
    public static int finalmente(int n) {
        int r = 0;
        try {
            r = 100 / n;
        } catch (ArithmeticException e) {
            r = 5;
        } catch (RuntimeException e) {
            r = 6;
        } finally {
            r = r + 2;
        }
        return r;
    }

    // El `finally` con el valor pendiente (§76): el `return` de adentro guarda el 10 en un hueco,
    // copia el bloque —que escribe otro local— y recarga. Son 10 por el `return` y 101 por la caida.
    public static int salidaPendiente(int n) {
        int r = 0;
        try {
            if (n > 0) {
                return n * 2;
            }
            r = n + 1;
        } finally {
            r = r + 100;
        }
        return r;
    }

    // El punto flotante y el `instanceof` en ART (párrafos 77 a 80): cuenta con promoción de entero a
    // doble, campo `float` leido con la forma de una palabra, conversion a largo y dos pruebas de
    // tipo. Con n = 2 son 6 + 1 + 1 = 8.
    static double doble = 2.5;
    static float flotante = 1.5f;

    public static int flotantes(int n) {
        double d = n * doble + flotante;
        long l = (long) d;
        Object o = "cadena";
        if (o instanceof String) {
            l = l + 1;
        }
        if (o instanceof CharSequence) {
            l = l + 1;
        }
        return (int) l;
    }

    // Un `double` que sale por `println`: la constante 1.5 va al pool como `Double` (dos posiciones) y
    // el entero se ensancha en su propio sitio, antes de la cuenta.
    public static double dobleCon(int n) {
        return n * 1.5;
    }
}
