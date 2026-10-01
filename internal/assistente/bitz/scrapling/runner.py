"""Bitz UI driver. Credentials arrive only through stdin; stdout is a JSON result.

Scrapling owns the browser lifecycle. No replay of an action containing a save.
The Vue model is READ ONLY, used to verify the UI actually accepted each action.
"""
import json
import logging
import re
import sys
import unicodedata
from datetime import datetime
from urllib.parse import urljoin, urlsplit

from scrapling.fetchers import DynamicFetcher


def normalized(value):
    return ' '.join(''.join(c for c in unicodedata.normalize('NFD', value)
                           if not unicodedata.combining(c)).lower().split())


class FlowError(Exception):
    pass


class Flow:
    def __init__(self, request):
        self.request = request
        self.stage = 'login'
        self.result = {'ok': False, 'stage': self.stage, 'code': 'not_started'}

    def run(self, page):
        try:
            self.perform(page)
        except FlowError as exc:
            self.result = {'ok': False, 'stage': self.stage, 'code': str(exc)}
        except Exception:
            # Browser errors can contain field values, page text or URLs.
            self.result = {'ok': False, 'stage': self.stage, 'code': 'browser_or_timeout'}

    def perform(self, page):
        r = self.request
        page.set_default_timeout(30000)
        page.locator('#username').fill(r['username'])
        page.locator('#password').fill(r['password'])
        page.locator('#loginButton').click()
        page.locator('#btn-add-reserva').wait_for(state='visible')
        if r['mode'] == 'login':
            self.result = {'ok': True, 'stage': 'login', 'code': 'authenticated'}
            return

        self.stage = 'identity'
        modal = page.locator('#modal-reserva')
        for _ in range(3):
            page.locator('#btn-add-reserva').click()
            try:
                modal.locator('#reserva_cpf').wait_for(state='visible', timeout=3000)
                break
            except Exception:
                pass
        modal.locator('#reserva_cpf').fill(r['cpf'])
        modal.locator('[title="Pesquisar por CPF"]').click()
        page.wait_for_function('Number(vueReserva.reserva.pessoa_id) > 0')
        modal.locator('#btn-avancar').click()

        self.stage = 'dates'
        for selector, value in [('#reserva_data_reserva', r['checkin']),
                                ('#reserva_data_saida', r['checkout'])]:
            field = modal.locator(selector)
            field.wait_for(state='visible')
            field.press('ControlOrMeta+A')
            field.press_sequentially(value, delay=70)
            field.press('Tab')
            if field.input_value() != value:
                raise FlowError('date_not_accepted')
        self.stage = 'dates_status'
        modal.get_by_role('button', name=re.compile(r'Pré Reserva', re.I)).click()
        modal.get_by_role('button', name=re.compile(r'Aberto', re.I)).click()
        self.verify_model(page, [], [])
        self.stage = 'sales_channel'
        modal.locator('#btn-avancar').click()
        # The actual wizard has a sales-channel step between dates and UHs.
        modal.locator('#reserva_canalVenda').wait_for(state='visible')
        modal.locator('#btn-avancar').click()
        modal.locator('#btn-add-quarto-reserva').wait_for(state='visible')

        self.stage = 'rooms'
        modal.locator('#btn-add-quarto-reserva').click()
        picker = page.locator('#modal-quarto-reserva')
        picker.wait_for(state='visible')
        rows = picker.locator('#table-quartos-disponiveis-reserva tr')
        rows.first.wait_for(state='visible')
        selected = []
        for category in r['categories']:
            page.wait_for_function('''needle=>[...document.querySelectorAll('#modal-quarto-reserva.in #table-quartos-disponiveis-reserva tr')]
                .some(row=>{const cell=row.children[2];const text=(cell?.innerText||'').normalize('NFD')
                .replace(/[\\u0300-\\u036f]/g,'').toLowerCase().trim().replace(/\\s+/g,' ');return text===needle})''',
                arg=normalized(category), timeout=15000)
            chosen = None
            for row in rows.all():
                cells = row.locator('td')
                if cells.count() < 3:
                    continue
                number = re.match(r'\s*(\d+)', cells.nth(1).inner_text())
                if not number or number[1] in selected:
                    continue
                if normalized(cells.nth(2).inner_text()) == normalized(category):
                    chosen = (row, number[1])
                    break
            if chosen is None:
                raise FlowError('category_unavailable')
            row, number = chosen
            row.locator('td').first.click()
            row.locator('td:first-child i.fa-check-square').wait_for(state='visible')
            selected.append(number)
        picker.locator('#btn-salvar-quarto-reserva').click()
        picker.wait_for(state='hidden')
        page.wait_for_function('(n)=>vueReserva.reserva.quartosSelecionados.length===n', arg=len(selected))
        self.verify_model(page, selected, r['categories'])

        self.stage = 'review'
        modal.locator('#btn-avancar').click()
        modal.locator('#reserva_valor_sinal').wait_for(state='visible')
        modal.locator('#btn-avancar').click()
        modal.locator('#btn-salvar-p').wait_for(state='visible')
        self.verify_model(page, selected, r['categories'])
        if r['mode'] == 'probe':
            self.result = {'ok': True, 'stage': 'review', 'code': 'verified_without_save', 'rooms': selected}
            return

        self.stage = 'save'
        save_url = urljoin(page.url, page.evaluate('vueReserva.urlSave'))
        if urlsplit(save_url).netloc != urlsplit(r['url']).netloc:
            raise FlowError('unexpected_save_origin')
        # Exactly one final click. A timeout is ambiguous; never retry a save.
        with page.expect_response(lambda response: response.url == save_url
                                  and response.request.method == 'POST', timeout=60000) as saved:
            modal.locator('#btn-salvar-p').click()
        response = saved.value
        data = response.json()
        if not response.ok or data.get('status') is not True or not data.get('id') or not data.get('codigo'):
            raise FlowError('save_not_confirmed')
        page.wait_for_function('(id)=>String(vueReserva.reserva.id)===String(id) && vueReserva.enableSaveButton===false', arg=data['id'])
        self.stage = 'completion'
        page.wait_for_timeout(20000)
        self.result = {'ok': True, 'stage': 'completion', 'code': 'saved',
                       'reference': str(data['codigo']), 'rooms': selected}

    def verify_model(self, page, selected, categories):
        state = page.evaluate('''()=>{const r=vueReserva.reserva;return {
            checkin:r.data_reserva,checkout:r.data_saida,status:r.situacao,payment:r.forma_pagamento,
            rooms:r.quartosSelecionados.map(q=>({category:String(q.quarto.descricao||''),checkin:q.checkin,checkout:q.checkout}))}}''')
        if (state['checkin'], state['checkout'], state['status'], state['payment']) != (
                self.request['checkin'], self.request['checkout'], 'P', 'A'):
            raise FlowError('reservation_state_mismatch')
        if len(state['rooms']) != len(selected):
            raise FlowError('selected_rooms_mismatch')
        if sorted(normalized(q['category']) for q in state['rooms']) != sorted(normalized(c) for c in categories):
            raise FlowError('selected_categories_mismatch')
        for q in state['rooms']:
            if q['checkin'] != self.request['checkin'] or q['checkout'] != self.request['checkout']:
                raise FlowError('room_dates_mismatch')


def execute(request):
    if request.get('mode') not in ('login', 'probe', 'create'):
        return {'ok': False, 'stage': 'input', 'code': 'invalid_mode'}
    if request['mode'] != 'login':
        try:
            start = datetime.strptime(request['checkin'], '%d/%m/%Y')
            end = datetime.strptime(request['checkout'], '%d/%m/%Y')
            if end <= start or not 1 <= len(request['categories']) <= 6:
                raise ValueError()
        except (KeyError, ValueError, TypeError):
            return {'ok': False, 'stage': 'input', 'code': 'invalid_reservation'}
    flow = Flow(request)
    try:
        DynamicFetcher.fetch(request['url'].rstrip('/') + '/login', page_action=flow.run,
                             executable_path=request['browser'], headless=True,
                             google_search=False, retries=1, timeout=30000)
    except Exception:
        return {'ok': False, 'stage': flow.stage, 'code': 'browser_or_timeout'}
    # Scrapling logs/swallow page_action exceptions: never infer success from HTTP 200.
    return flow.result


if __name__ == '__main__':
    logging.disable(logging.CRITICAL)
    try:
        result = execute(json.load(sys.stdin))
    except Exception:
        result = {'ok': False, 'stage': 'input', 'code': 'invalid_input'}
    print(json.dumps(result, ensure_ascii=True), flush=True)
    sys.exit(0 if result.get('ok') else 1)
