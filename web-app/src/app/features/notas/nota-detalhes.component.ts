import { Component, OnInit, OnDestroy, inject, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { FormsModule } from '@angular/forms';
import { interval, switchMap, takeWhile, timeout, catchError, of, Subscription } from 'rxjs';
import { NotaFiscalService } from '../../core/services/nota-fiscal.service';
import { ProdutoService } from '../../core/services/produto.service';
import { IdempotenciaService } from '../../core/services/idempotencia.service';
import { NotaFiscal, AdicionarItemRequest } from '../../core/models/nota-fiscal.model';
import { Produto } from '../../core/models/produto.model';

@Component({
  selector: 'app-nota-detalhes',
  standalone: true,
  imports: [CommonModule, FormsModule, RouterLink],
  template: `
    <div class="container mx-auto px-4 py-8">
      <div class="mb-6">
        <a routerLink="/notas" class="text-blue-600 hover:text-blue-800 flex items-center gap-2">
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7"/>
          </svg>
          Voltar para lista
        </a>
      </div>

      @if (carregando()) {
        <div class="text-center py-8">
          <div class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
          <p class="mt-2 text-gray-600">Carregando nota...</p>
        </div>
      }

      @if (nota()) {
        <div class="bg-white border rounded-lg p-6 shadow-sm mb-6">
          <div class="flex justify-between items-start mb-4">
            <div>
              <h1 class="text-2xl font-bold text-gray-800">{{ nota()!.numero }}</h1>
              <p class="text-sm text-gray-600 mt-1">ID: {{ nota()!.id }}</p>
            </div>
            <span class="px-3 py-1 rounded-full text-sm font-medium"
                  [class.bg-yellow-100]="nota()!.status === 'ABERTA'"
                  [class.text-yellow-800]="nota()!.status === 'ABERTA'"
                  [class.bg-green-100]="nota()!.status === 'FECHADA'"
                  [class.text-green-800]="nota()!.status === 'FECHADA'">
              {{ nota()!.status }}
            </span>
          </div>

          <div class="grid grid-cols-2 gap-4 text-sm">
            <div>
              <span class="text-gray-600">Data Criação:</span>
              <span class="ml-2 font-medium">{{ nota()!.dataCriacao | date:'dd/MM/yyyy HH:mm' }}</span>
            </div>
            @if (nota()!.dataFechada) {
              <div>
                <span class="text-gray-600">Data Fechamento:</span>
                <span class="ml-2 font-medium">{{ nota()!.dataFechada | date:'dd/MM/yyyy HH:mm' }}</span>
              </div>
            }
          </div>
        </div>

        <!-- Itens da Nota -->
        <div class="bg-white border rounded-lg p-6 shadow-sm mb-6">
          <h2 class="text-xl font-semibold mb-4">Itens da Nota</h2>

          @if (nota()!.itens && nota()!.itens!.length > 0) {
            <div class="overflow-x-auto">
              <table class="w-full text-sm">
                <thead class="bg-gray-50 border-b">
                  <tr>
                    <th class="text-left p-3">Produto ID</th>
                    <th class="text-right p-3">Quantidade</th>
                    <th class="text-right p-3">Preço Unit.</th>
                    <th class="text-right p-3">Subtotal</th>
                  </tr>
                </thead>
                <tbody>
                  @for (item of nota()!.itens; track item.id) {
                    <tr class="border-b">
                      <td class="p-3 font-mono text-xs">{{ item.produtoId }}</td>
                      <td class="p-3 text-right">{{ item.quantidade }}</td>
                      <td class="p-3 text-right">R$ {{ item.precoUnitario | number:'1.2-2' }}</td>
                      <td class="p-3 text-right font-medium">
                        R$ {{ (item.quantidade * item.precoUnitario) | number:'1.2-2' }}
                      </td>
                    </tr>
                  }
                </tbody>
                <tfoot class="bg-gray-50">
                  <tr>
                    <td colspan="3" class="p-3 text-right font-semibold">Total:</td>
                    <td class="p-3 text-right font-bold text-lg">R$ {{ calcularTotal() | number:'1.2-2' }}</td>
                  </tr>
                </tfoot>
              </table>
            </div>
          } @else {
            <p class="text-gray-500 text-center py-4">Nenhum item adicionado</p>
          }
        </div>

        <!-- Adicionar Item -->
        @if (nota()!.status === 'ABERTA') {
          <div class="bg-white border rounded-lg p-6 shadow-sm mb-6">
            <h2 class="text-xl font-semibold mb-4">Adicionar Item</h2>

            <form (ngSubmit)="adicionarItem()" #form="ngForm">
              <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
                <div>
                  <label class="block text-sm font-medium text-gray-700 mb-1">Produto</label>
                  <select
                    [(ngModel)]="novoItem.produtoId"
                    name="produtoId"
                    required
                    class="w-full px-3 py-2 border rounded-lg focus:ring-2 focus:ring-blue-500">
                    <option value="">Selecione...</option>
                    @for (produto of produtos(); track produto.id) {
                      <option [value]="produto.id">{{ produto.nome }} ({{ produto.sku }})</option>
                    }
                  </select>
                </div>

                <div>
                  <label class="block text-sm font-medium text-gray-700 mb-1">Quantidade</label>
                  <input
                    type="number"
                    [(ngModel)]="novoItem.quantidade"
                    name="quantidade"
                    required
                    min="1"
                    class="w-full px-3 py-2 border rounded-lg focus:ring-2 focus:ring-blue-500"
                  />
                </div>

                <div>
                  <label class="block text-sm font-medium text-gray-700 mb-1">Preço Unitário</label>
                  <input
                    type="number"
                    [(ngModel)]="novoItem.precoUnitario"
                    name="precoUnitario"
                    required
                    min="0"
                    step="0.01"
                    class="w-full px-3 py-2 border rounded-lg focus:ring-2 focus:ring-blue-500"
                  />
                </div>
              </div>

              @if (erroItem()) {
                <div class="mt-4 p-3 bg-red-50 border border-red-200 rounded-lg">
                  <p class="text-sm text-red-800">{{ erroItem() }}</p>
                </div>
              }

              <button
                type="submit"
                [disabled]="!form.valid || adicionandoItem()"
                class="mt-4 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:bg-gray-400 disabled:cursor-not-allowed transition">
                {{ adicionandoItem() ? 'Adicionando...' : 'Adicionar Item' }}
              </button>
            </form>
          </div>

          <!-- Botão Imprimir -->
          <div class="bg-white border rounded-lg p-6 shadow-sm">
            <h2 class="text-xl font-semibold mb-4">Impressão</h2>

            @if (statusImpressao() === 'aguardando') {
              <div class="flex items-center gap-3 p-4 bg-blue-50 border border-blue-200 rounded-lg">
                <div class="animate-spin rounded-full h-6 w-6 border-b-2 border-blue-600"></div>
                <p class="text-blue-800">Processando impressão... aguarde</p>
              </div>
            } @else if (statusImpressao() === 'sucesso') {
              <div class="p-4 bg-green-50 border border-green-200 rounded-lg">
                <p class="text-green-800 font-medium">✓ Nota impressa com sucesso!</p>
              </div>
            } @else if (statusImpressao() === 'falha') {
              <div class="p-4 bg-red-50 border border-red-200 rounded-lg">
                <p class="text-red-800 font-medium">✗ Falha na impressão: {{ mensagemErro() }}</p>
              </div>
            } @else {
              <button
                (click)="solicitarImpressao()"
                [disabled]="!nota()!.itens || nota()!.itens!.length === 0"
                class="px-6 py-3 bg-green-600 text-white rounded-lg hover:bg-green-700 disabled:bg-gray-400 disabled:cursor-not-allowed transition font-medium">
                🖨️ Solicitar Impressão
              </button>
              @if (!nota()!.itens || nota()!.itens!.length === 0) {
                <p class="text-sm text-gray-500 mt-2">Adicione ao menos um item para imprimir</p>
              }
            }
          </div>
        }
      }
    </div>
  `
})
export class NotaDetalhesComponent implements OnInit, OnDestroy {
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private readonly notaService = inject(NotaFiscalService);
  private readonly produtoService = inject(ProdutoService);
  private readonly idempotenciaService = inject(IdempotenciaService);

  nota = signal<NotaFiscal | null>(null);
  produtos = signal<Produto[]>([]);
  carregando = signal(false);
  adicionandoItem = signal(false);
  erroItem = signal<string | null>(null);
  statusImpressao = signal<'idle' | 'aguardando' | 'sucesso' | 'falha'>('idle');
  mensagemErro = signal<string | null>(null);

  novoItem: AdicionarItemRequest = {
    produtoId: '',
    quantidade: 1,
    precoUnitario: 0
  };

  private pollingSub?: Subscription;

  ngOnInit(): void {
    const id = this.route.snapshot.paramMap.get('id');
    if (id) {
      this.carregarNota(id);
      this.carregarProdutos();
    }
  }

  ngOnDestroy(): void {
    this.pollingSub?.unsubscribe();
  }

  carregarNota(id: string): void {
    this.carregando.set(true);
    this.notaService.buscarNota(id).subscribe({
      next: (nota) => {
        this.nota.set(nota);
        this.carregando.set(false);
      },
      error: (err) => {
        console.error('Erro ao carregar nota:', err);
        this.carregando.set(false);
      }
    });
  }

  carregarProdutos(): void {
    this.produtoService.listarProdutos().subscribe({
      next: (produtos) => this.produtos.set(produtos),
      error: (err) => console.error('Erro ao carregar produtos:', err)
    });
  }

  adicionarItem(): void {
    const notaId = this.nota()?.id;
    if (!notaId) return;

    this.erroItem.set(null);
    this.adicionandoItem.set(true);

    this.notaService.adicionarItem(notaId, this.novoItem).subscribe({
      next: () => {
        this.adicionandoItem.set(false);
        this.novoItem = { produtoId: '', quantidade: 1, precoUnitario: 0 };
        this.carregarNota(notaId);
      },
      error: (err) => {
        this.adicionandoItem.set(false);
        this.erroItem.set(err.error?.erro || 'Erro ao adicionar item');
      }
    });
  }

  solicitarImpressao(): void {
    const notaId = this.nota()?.id;
    if (!notaId) return;

    const chave = this.idempotenciaService.gerarChave();
    this.statusImpressao.set('aguardando');
    this.mensagemErro.set(null);

    this.notaService.imprimirNota(notaId, chave).subscribe({
      next: (resposta) => {
        this.iniciarPolling(resposta.id);
      },
      error: (err) => {
        this.statusImpressao.set('falha');
        this.mensagemErro.set(err.error?.erro || 'Erro ao solicitar impressão');
      }
    });
  }

  iniciarPolling(solicitacaoId: string): void {
    this.pollingSub = interval(1000).pipe(
      switchMap(() => this.notaService.consultarStatusImpressao(solicitacaoId)),
      timeout(30000),
      takeWhile((sol) => sol.status === 'PENDENTE', true),
      catchError((err) => {
        this.statusImpressao.set('falha');
        this.mensagemErro.set('Timeout aguardando resposta');
        return of(null);
      })
    ).subscribe({
      next: (sol) => {
        if (!sol) return;

        if (sol.status === 'CONCLUIDA') {
          this.statusImpressao.set('sucesso');
          this.carregarNota(this.nota()!.id);
        } else if (sol.status === 'FALHOU') {
          this.statusImpressao.set('falha');
          this.mensagemErro.set(sol.mensagemErro || 'Erro desconhecido');
        }
      }
    });
  }

  calcularTotal(): number {
    const itens = this.nota()?.itens || [];
    return itens.reduce((total, item) => total + (item.quantidade * item.precoUnitario), 0);
  }
}
