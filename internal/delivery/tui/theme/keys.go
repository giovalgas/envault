package theme

import (
	"github.com/charmbracelet/bubbles/key"
)

type KeyMap struct {
	Up         key.Binding
	Down       key.Binding
	Mark       key.Binding
	Compose    key.Binding
	Open       key.Binding
	Reveal     key.Binding
	Copy       key.Binding
	New        key.Binding
	Edit       key.Binding
	Duplicate  key.Binding
	Rename     key.Binding
	Import     key.Binding
	Delete     key.Binding
	MoveUp     key.Binding
	MoveDown   key.Binding
	Expand     key.Binding
	NextPane   key.Binding
	PrevPane   key.Binding
	Write      key.Binding
	Target     key.Binding
	Clipboard  key.Binding
	Filter     key.Binding
	Help       key.Binding
	Quit       key.Binding
	Back       key.Binding
	Confirm    key.Binding
	NextChoice key.Binding
	PrevChoice key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:         key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "subir")),
		Down:       key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "descer")),
		Mark:       key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "marcar")),
		Compose:    key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "carregar marcadas")),
		Open:       key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "detalhe")),
		Reveal:     key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "revelar/ocultar")),
		Copy:       key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "copiar valor")),
		New:        key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "nova env")),
		Edit:       key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "editar")),
		Duplicate:  key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "duplicar")),
		Rename:     key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "renomear")),
		Import:     key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "importar .env")),
		Delete:     key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "apagar")),
		MoveUp:     key.NewBinding(key.WithKeys("K"), key.WithHelp("K", "subir na montagem")),
		MoveDown:   key.NewBinding(key.WithKeys("J"), key.WithHelp("J", "descer na montagem")),
		Expand:     key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "expandir conflito")),
		NextPane:   key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "próximo painel")),
		PrevPane:   key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "painel anterior")),
		Write:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "carregar montagem")),
		Target:     key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "terminal ou arquivo")),
		Clipboard:  key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "copiar .env")),
		Filter:     key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filtrar")),
		Help:       key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "ajuda")),
		Quit:       key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q/ctrl+c", "sair/voltar")),
		Back:       key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "voltar")),
		Confirm:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirmar")),
		NextChoice: key.NewBinding(key.WithKeys("tab", "right"), key.WithHelp("tab", "próxima opção")),
		PrevChoice: key.NewBinding(key.WithKeys("shift+tab", "left"), key.WithHelp("shift+tab", "opção anterior")),
	}
}

type HelpBindings struct {
	short []key.Binding
	full  [][]key.Binding
}

func (h HelpBindings) ShortHelp() []key.Binding {
	return h.short
}

func (h HelpBindings) FullHelp() [][]key.Binding {
	return h.full
}

func (k KeyMap) ListHelp() HelpBindings {
	return HelpBindings{
		short: []key.Binding{k.Mark, k.Compose, k.New, k.Filter, k.Help, k.Quit},
		full:  k.AllGroups(),
	}
}

func (k KeyMap) FilterHelp() HelpBindings {
	return HelpBindings{
		short: []key.Binding{k.Confirm, k.Back},
		full:  k.AllGroups(),
	}
}

func (k KeyMap) DetailHelp() HelpBindings {
	return HelpBindings{
		short: []key.Binding{k.Up, k.Down, k.Reveal, k.Copy, k.Help, k.Back},
		full:  k.AllGroups(),
	}
}

func (k KeyMap) ComposeHelp() HelpBindings {
	return HelpBindings{
		short: []key.Binding{k.MoveUp, k.MoveDown, k.NextPane, k.Write, k.Help, k.Back},
		full:  k.AllGroups(),
	}
}

func (k KeyMap) ModalHelp() HelpBindings {
	return HelpBindings{
		short: []key.Binding{k.Confirm, k.NextChoice, k.Back},
		full:  k.AllGroups(),
	}
}

func (k KeyMap) HelpScreenHelp() HelpBindings {
	return HelpBindings{
		short: []key.Binding{k.Help, k.Back},
		full:  k.AllGroups(),
	}
}

func (k KeyMap) AllGroups() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Mark, k.Compose, k.Open},
		{k.Reveal, k.Copy},
		{k.New, k.Edit, k.Duplicate, k.Rename, k.Import, k.Delete},
		{k.MoveUp, k.MoveDown, k.NextPane, k.Expand, k.Target, k.Write, k.Clipboard},
		{k.Filter, k.Help, k.Quit, k.Back},
	}
}
