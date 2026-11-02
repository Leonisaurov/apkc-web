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
}
