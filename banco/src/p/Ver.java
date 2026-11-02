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
            i = i + 1;
        }
    }

    public static void elige(int n) {
        switch (n) {
            case 0: total = total + 10; break;
            case 1: total = total + 20; break;
            default: total = total + 1;
        }
    }
}
