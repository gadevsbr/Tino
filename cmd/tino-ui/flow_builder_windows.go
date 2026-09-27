package main

import (
	"fmt"
	"strings"

	"github.com/gadevsbr/tino/internal/flow"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

type ruleListModel struct {
	walk.ListModelBase
	rules []flow.Rule
}

func (m *ruleListModel) ItemCount() int { return len(m.rules) }
func (m *ruleListModel) Value(index int) interface{} {
	if index < 0 || index >= len(m.rules) {
		return ""
	}
	r := m.rules[index]
	return fmt.Sprintf("%02d  %s   •   contém “%s”", index+1, r.Name, truncate(r.Contains, 34))
}
func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}

func (a *application) syncFlowModel(selectIndex int) {
	a.flowModel.rules = append(a.flowModel.rules[:0], a.flowDef.Rules...)
	a.flowModel.PublishItemsReset()
	if len(a.flowDef.Rules) == 0 {
		a.flowList.SetCurrentIndex(-1)
		return
	}
	if selectIndex < 0 {
		selectIndex = 0
	}
	if selectIndex >= len(a.flowDef.Rules) {
		selectIndex = len(a.flowDef.Rules) - 1
	}
	a.flowList.SetCurrentIndex(selectIndex)
}

func (a *application) currentFlowDefinition() flow.Definition {
	def := a.flowDef
	def.Rules = append([]flow.Rule(nil), a.flowDef.Rules...)
	def.DefaultReply = strings.TrimSpace(a.defaultReply.Text())
	return def
}

func (a *application) addRule() {
	rule, ok := a.ruleDialog("Nova regra de atendimento", flow.Rule{})
	if !ok {
		return
	}
	a.flowDef.Rules = append(a.flowDef.Rules, rule)
	a.syncFlowModel(len(a.flowDef.Rules) - 1)
	a.appendLog("Nova regra adicionada: " + rule.Name)
}

func (a *application) editRule() {
	index := a.flowList.CurrentIndex()
	if index < 0 || index >= len(a.flowDef.Rules) {
		walk.MsgBox(a.mw, "Editar regra", "Selecione uma regra primeiro.", walk.MsgBoxIconInformation)
		return
	}
	rule, ok := a.ruleDialog("Editar regra de atendimento", a.flowDef.Rules[index])
	if !ok {
		return
	}
	a.flowDef.Rules[index] = rule
	a.syncFlowModel(index)
	a.appendLog("Regra atualizada: " + rule.Name)
}

func (a *application) removeRule() {
	index := a.flowList.CurrentIndex()
	if index < 0 || index >= len(a.flowDef.Rules) {
		walk.MsgBox(a.mw, "Excluir regra", "Selecione uma regra primeiro.", walk.MsgBoxIconInformation)
		return
	}
	name := a.flowDef.Rules[index].Name
	if walk.MsgBox(a.mw, "Excluir regra", "Remover a regra “"+name+"”?", walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != walk.DlgCmdYes {
		return
	}
	a.flowDef.Rules = append(a.flowDef.Rules[:index], a.flowDef.Rules[index+1:]...)
	a.syncFlowModel(index)
	a.appendLog("Regra removida: " + name)
}

func (a *application) moveRule(delta int) {
	index := a.flowList.CurrentIndex()
	target := index + delta
	if index < 0 || target < 0 || target >= len(a.flowDef.Rules) {
		return
	}
	a.flowDef.Rules[index], a.flowDef.Rules[target] = a.flowDef.Rules[target], a.flowDef.Rules[index]
	a.syncFlowModel(target)
}

func (a *application) saveFlow() {
	def := a.currentFlowDefinition()
	if err := flow.SaveDefinition(a.cfg.Flow.RulesFile, def); err != nil {
		walk.MsgBox(a.mw, "Não foi possível salvar", err.Error(), walk.MsgBoxIconError)
		return
	}
	a.flowDef = def
	a.appendLog(fmt.Sprintf("Fluxo salvo com %d regra(s).", len(def.Rules)))
	walk.MsgBox(a.mw, "Fluxo salvo", "As regras do atendimento foram salvas com sucesso.", walk.MsgBoxIconInformation)
}

func (a *application) testFlow() {
	message := strings.TrimSpace(a.testMessage.Text())
	if message == "" {
		walk.MsgBox(a.mw, "Teste do fluxo", "Digite uma mensagem de exemplo.", walk.MsgBoxIconInformation)
		return
	}
	reply := flow.Match(a.currentFlowDefinition(), message)
	if reply == "" {
		reply = "Nenhuma resposta seria enviada."
	}
	a.testResult.SetText("Resposta simulada:\r\n" + reply)
}

func (a *application) ruleDialog(title string, initial flow.Rule) (flow.Rule, bool) {
	var dlg *walk.Dialog
	var name, contains *walk.LineEdit
	var reply *walk.TextEdit
	var sensitive *walk.CheckBox
	var accept, cancel *walk.PushButton
	result := initial
	err := Dialog{
		AssignTo: &dlg, Title: title, Icon: a.icon, Size: Size{Width: 560, Height: 460}, FixedSize: true,
		DefaultButton: &accept, CancelButton: &cancel, Layout: VBox{Margins: Margins{Left: 20, Top: 18, Right: 20, Bottom: 18}, Spacing: 8},
		Children: []Widget{
			Label{Text: "Nome da regra", Font: Font{Bold: true}}, LineEdit{AssignTo: &name, Text: initial.Name, CueBanner: "Ex.: Horário de atendimento"},
			Label{Text: "Quando a mensagem contiver", Font: Font{Bold: true}}, LineEdit{AssignTo: &contains, Text: initial.Contains, CueBanner: "Ex.: horário"},
			Label{Text: "Responder com", Font: Font{Bold: true}}, TextEdit{AssignTo: &reply, Text: initial.Reply, MinSize: Size{Height: 120}},
			CheckBox{AssignTo: &sensitive, Text: "Diferenciar maiúsculas e minúsculas", Checked: initial.CaseSensitive},
			VSpacer{},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{HSpacer{}, PushButton{AssignTo: &cancel, Text: "Cancelar", OnClicked: func() { dlg.Cancel() }}, PushButton{AssignTo: &accept, Text: "Salvar regra", MinSize: Size{Width: 130}, Font: Font{Bold: true}, OnClicked: func() {
				candidate := flow.Rule{Name: strings.TrimSpace(name.Text()), Contains: strings.TrimSpace(contains.Text()), Reply: strings.TrimSpace(reply.Text()), CaseSensitive: sensitive.Checked()}
				if candidate.Name == "" || candidate.Contains == "" || candidate.Reply == "" {
					walk.MsgBox(dlg, "Campos obrigatórios", "Preencha o nome, a condição e a resposta.", walk.MsgBoxIconWarning)
					return
				}
				result = candidate
				dlg.Accept()
			}}}},
		},
	}.Create(a.mw)
	if err != nil {
		walk.MsgBox(a.mw, "Editor de regra", err.Error(), walk.MsgBoxIconError)
		return initial, false
	}
	return result, dlg.Run() == walk.DlgCmdOK
}
