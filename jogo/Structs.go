package main

// Comando define a requisição enviada do Cliente para o Servidor.
type Comando struct {
    ClientID        string 
    SequenceNumber  int   
    Acao            string 
    Detalhe         string 
}

// EstadoJogador representa a posição e vida de um jogador.
type EstadoJogador struct {
    X, Y          int
    Vidas         int
    UltimoComando int 
}

// EstadoJogo representa o estado completo que o servidor mantém.
type EstadoJogo struct {
    Jogadores map[string]EstadoJogador
}

// Resposta define a resposta retornada do Servidor para o Cliente.
type Resposta struct {
    Sucesso     bool
    Mensagem    string
    EstadoAtual EstadoJogo 
}

// Funções construtoras para elementos móveis do jogo

func NovoPortal() *Portal {
    return &Portal{
        Elemento: Elemento{
            simbolo: 'P', 
            tangivel: false,
        },
        PararTeletransporte: make(chan bool),
    }
}

func NovaArmadilha() *Armadilha {
    return &Armadilha{
        Elemento: Elemento{
            simbolo: 'A', 
            tangivel: false,
        },
        ProximidadeJogador: make(chan bool),
        ProximidadeOutro:   make(chan bool),
        PararArmadilha:     make(chan bool),
    }
}

func NovoGuarda() *Guarda {
    return &Guarda{
        Elemento: Elemento{
            simbolo: 'G',  
            tangivel: false,
        },
        PararPerseguicao: make(chan bool),
        Perseguir:         make(chan bool),
    }
}
