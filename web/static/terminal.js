const term = new Terminal({
  cursorBlink: true,
  convertEol: true,
  fontSize: 14,
  fontFamily: "'Fira Code', 'Cascadia Code', 'JetBrains Mono', monospace",
  theme: {
    background: '#0a0a0a',
    foreground: '#c8c8c8',
    cursor: '#ffffff',
    selectionBackground: '#404040',
    black: '#1e1e1e',
    red: '#d16969',
    green: '#6a9955',
    yellow: '#d7ba7d',
    blue: '#569cd6',
    magenta: '#c586c0',
    cyan: '#4ec9b0',
    white: '#d4d4d4',
  },
});

const fitAddon = new FitAddon.FitAddon();
term.loadAddon(fitAddon);
term.loadAddon(new WebLinksAddon.WebLinksAddon());

term.open(document.getElementById('terminal'));
fitAddon.fit();

let inputBuffer = '';
let ws = null;

function connect() {
  const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
  ws = new WebSocket(`${protocol}//${location.host}/ws`);

  ws.onopen = () => {
    term.clear();
    term.write('CLI-Game: MUD Narrativo\r\n\r\n');
    term.write('usuario: ');
  };

  ws.onmessage = (event) => {
    if (loginStep === 'done') {
      term.reset();
    }
    term.write(event.data);
  };

  ws.onclose = () => {
    term.write('\r\n\x1b[31mConexao encerrada. Recarregue para jogar.\x1b[0m\r\n');
  };

  ws.onerror = () => {
    term.write('\r\n\x1b[31mErro de conexao.\x1b[0m\r\n');
  };
}

let loginStep = 'username';
let username = '';

term.onData((data) => {
  if (data === '\r') {
    // Enter pressed
    if (ws && ws.readyState === WebSocket.OPEN) {
      if (loginStep === 'username') {
        username = inputBuffer;
        inputBuffer = '';
        term.write('\r\nsenha: ');
        loginStep = 'password';
      } else if (loginStep === 'password') {
        ws.send(JSON.stringify({
          type: 'login',
          username: username,
          password: inputBuffer,
        }));
        inputBuffer = '';
        loginStep = 'done';
      } else {
        ws.send(inputBuffer);
        inputBuffer = '';
      }
    }
  } else if (data === '\x7f') {
    // Backspace
    if (inputBuffer.length > 0) {
      inputBuffer = inputBuffer.slice(0, -1);
      term.write('\b \b');
    }
  } else if (data === '\x03') {
    // Ctrl+C
    inputBuffer = '';
    term.write('^C\r\n');
  } else if (data >= ' ') {
    inputBuffer += data;
    if (loginStep === 'password') {
      term.write('*');
    } else {
      term.write(data);
    }
  }
});

window.addEventListener('resize', () => {
  fitAddon.fit();
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }));
  }
});

connect();
