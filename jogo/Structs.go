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