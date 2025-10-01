import { Component, OnInit, inject, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterLink } from '@angular/router';
import { ProdutoService } from '../../core/services/produto.service';
import { Produto } from '../../core/models/produto.model';
import { ProdutoFormComponent } from './produto-form.component';

@Component({
  selector: 'app-produtos-lista',
  standalone: true,
  imports: [CommonModule, RouterLink, ProdutoFormComponent],
  template: `
    <div class="container mx-auto px-4 py-8">
      <div class="flex justify-between items-center mb-6">
        <h1 class="text-3xl font-bold text-gray-800">Produtos</h1>
        <button
          (click)="mostrarFormulario.set(!mostrarFormulario())"
          class="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition">
          {{ mostrarFormulario() ? 'Cancelar' : '+ Novo Produto' }}
        </button>
      </div>

      @if (mostrarFormulario()) {
        <app-produto-form
          (produtoCriado)="onProdutoCriado()"
          (cancelar)="mostrarFormulario.set(false)"
          class="block mb-6">
        </app-produto-form>
      }

      @if (carregando()) {
        <div class="text-center py-8">
          <div class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
          <p class="mt-2 text-gray-600">Carregando produtos...</p>
        </div>
      }

      @if (!carregando() && produtos().length === 0) {
        <div class="text-center py-12 bg-gray-50 rounded-lg">
          <p class="text-gray-500">Nenhum produto cadastrado</p>
        </div>
      }

      @if (!carregando() && produtos().length > 0) {
        <div class="grid gap-4">
          @for (produto of produtos(); track produto.id) {
            <div class="bg-white border rounded-lg p-4 shadow-sm hover:shadow-md transition">
              <div class="flex justify-between items-start">
                <div class="flex-1">
                  <div class="flex items-center gap-3">
                    <h3 class="text-lg font-semibold text-gray-800">{{ produto.nome }}</h3>
                    <span class="px-2 py-1 text-xs rounded-full"
                          [class.bg-green-100]="produto.ativo"
                          [class.text-green-800]="produto.ativo"
                          [class.bg-gray-100]="!produto.ativo"
                          [class.text-gray-800]="!produto.ativo">
                      {{ produto.ativo ? 'Ativo' : 'Inativo' }}
                    </span>
                  </div>
                  <p class="text-sm text-gray-600 mt-1">SKU: {{ produto.sku }}</p>
                  <div class="flex items-center gap-4 mt-2">
                    <span class="text-sm">
                      <strong>Saldo:</strong>
                      <span [class.text-red-600]="produto.saldo < 10"
                            [class.text-yellow-600]="produto.saldo >= 10 && produto.saldo < 50"
                            [class.text-green-600]="produto.saldo >= 50">
                        {{ produto.saldo }}
                      </span>
                    </span>
                    <span class="text-xs text-gray-500">
                      Criado em: {{ produto.dataCriacao | date:'dd/MM/yyyy HH:mm' }}
                    </span>
                  </div>
                </div>
              </div>
            </div>
          }
        </div>
      }
    </div>
  `
})
export class ProdutosListaComponent implements OnInit {
  private readonly produtoService = inject(ProdutoService);

  produtos = signal<Produto[]>([]);
  carregando = signal(false);
  mostrarFormulario = signal(false);

  ngOnInit(): void {
    this.carregarProdutos();
  }

  carregarProdutos(): void {
    this.carregando.set(true);
    this.produtoService.listarProdutos().subscribe({
      next: (produtos) => {
        this.produtos.set(produtos);
        this.carregando.set(false);
      },
      error: (err) => {
        console.error('Erro ao carregar produtos:', err);
        this.carregando.set(false);
      }
    });
  }

  onProdutoCriado(): void {
    this.mostrarFormulario.set(false);
    this.carregarProdutos();
  }
}
