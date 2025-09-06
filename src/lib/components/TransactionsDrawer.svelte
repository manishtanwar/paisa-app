<script lang="ts">
  import { createEventDispatcher, onMount } from "svelte";
  import { fly } from "svelte/transition";
  import { ajax, formatCurrency } from "$lib/utils";
  import type { Dayjs } from "dayjs";
  import dayjs from "dayjs";
  import Spinner from "./Spinner.svelte";

  export let isOpen: boolean = false;
  export let account: string = "";
  export let month: string = "";

  interface ExpenseTransaction {
    date: string;
    payee: string;
    account: string;
    amount: number;
    filename: string;
    beginLine: number;
    endLine: number;
  }

  let transactions: ExpenseTransaction[] = [];
  let filteredTransactions: ExpenseTransaction[] = [];
  let searchQuery: string = "";
  let loading: boolean = false;
  let total: number = 0;

  const dispatch = createEventDispatcher();

  $: if (isOpen && account) {
    loadTransactions();
  }

  $: {
    if (searchQuery) {
      filteredTransactions = transactions.filter(
        (t) =>
          t.payee.toLowerCase().includes(searchQuery.toLowerCase()) ||
          t.account.toLowerCase().includes(searchQuery.toLowerCase())
      );
    } else {
      filteredTransactions = transactions;
    }
  }

  async function loadTransactions() {
    loading = true;
    try {
      let url = `/api/expense/transactions?account=${encodeURIComponent(account)}`;
      if (month) {
        url += `&month=${month}`;
      }
      url += "&limit=100";

      const data = await ajax(url);
      transactions = data.transactions || [];
      total = data.total || 0;
    } catch (error) {
      console.error("Error loading transactions:", error);
      transactions = [];
    } finally {
      loading = false;
    }
  }

  function close() {
    isOpen = false;
    dispatch("close");
  }

  function handleKeyDown(e: KeyboardEvent) {
    if (e.key === "Escape") {
      close();
    }
  }

  function navigateToTransaction(t: ExpenseTransaction) {
    const filename = encodeURIComponent(t.filename);
    window.location.href = `/ledger/editor/${filename}?line=${t.beginLine}`;
  }

  function formatTransactionDate(dateStr: string): string {
    return dayjs(dateStr).format("DD MMM YYYY");
  }

  onMount(() => {
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("keydown", handleKeyDown);
    };
  });
</script>

{#if isOpen}
  <!-- Backdrop -->
  <div
    class="drawer-backdrop"
    on:click={close}
    on:keydown={handleKeyDown}
    role="button"
    tabindex="0"
  ></div>

  <!-- Drawer -->
  <div class="drawer" transition:fly={{ x: 400, duration: 300 }}>
    <div class="drawer-header">
      <div class="drawer-title">
        <span class="icon">
          <i class="fas fa-list"></i>
        </span>
        <div class="title-text">
          <h3>Transactions</h3>
          <span class="account-badge">{account}</span>
        </div>
      </div>
      <button class="delete is-medium" on:click={close} aria-label="Close"></button>
    </div>

    <div class="drawer-search">
      <div class="control has-icons-left">
        <input
          class="input"
          type="text"
          placeholder="Search transactions..."
          bind:value={searchQuery}
        />
        <span class="icon is-left">
          <i class="fas fa-search"></i>
        </span>
      </div>
    </div>

    <div class="drawer-content">
      {#if loading}
        <div class="loading-state">
          <Spinner />
          <p>Loading transactions...</p>
        </div>
      {:else if filteredTransactions.length === 0}
        <div class="empty-state">
          <span class="icon is-large">
            <i class="fas fa-inbox fa-2x"></i>
          </span>
          <p>No transactions found</p>
        </div>
      {:else}
        <div class="transactions-count">
          Showing {filteredTransactions.length} of {total} transactions
        </div>
        <div class="transactions-list">
          {#each filteredTransactions as t}
            <div
              class="transaction-item"
              on:click={() => navigateToTransaction(t)}
              on:keydown={(e) => e.key === "Enter" && navigateToTransaction(t)}
              role="button"
              tabindex="0"
            >
              <div class="transaction-header">
                <span class="transaction-date">
                  <i class="fas fa-calendar"></i>
                  {formatTransactionDate(t.date)}
                </span>
                <span class="transaction-amount" class:negative={t.amount < 0}>
                  {formatCurrency(t.amount)}
                </span>
              </div>
              <div class="transaction-payee">{t.payee}</div>
              <div class="transaction-account">{t.account}</div>
            </div>
          {/each}
        </div>
      {/if}
    </div>
  </div>
{/if}

<style lang="scss">
  .drawer-backdrop {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.5);
    z-index: 999;
  }

  .drawer {
    position: fixed;
    top: 0;
    right: 0;
    width: 450px;
    max-width: 100vw;
    height: 100vh;
    background: #ffffff;
    box-shadow: -4px 0 24px rgba(0, 0, 0, 0.2);
    z-index: 1000;
    display: flex;
    flex-direction: column;
  }

  :global(.dark) .drawer {
    background: #1a1a2e;
  }

  .drawer-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 1rem 1.25rem;
    border-bottom: 1px solid #e0e0e0;
    background: #f5f5f5;
  }

  :global(.dark) .drawer-header {
    background: #252540;
    border-color: #3a3a5a;
  }

  .drawer-title {
    display: flex;
    align-items: center;
    gap: 0.75rem;

    .icon {
      color: var(--bulma-primary);
      font-size: 1.25rem;
    }

    .title-text {
      h3 {
        margin: 0;
        font-size: 1.1rem;
        font-weight: 600;
        color: var(--bulma-text-strong);
      }

      .account-badge {
        font-size: 0.75rem;
        color: var(--bulma-text-weak);
      }
    }
  }

  .drawer-search {
    padding: 1rem 1.25rem;
    border-bottom: 1px solid var(--bulma-border-weak);
  }

  .drawer-content {
    flex: 1;
    overflow-y: auto;
    padding: 0;
  }

  .loading-state,
  .empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 200px;
    gap: 1rem;
    color: var(--bulma-text-weak);
  }

  .transactions-count {
    padding: 0.75rem 1.25rem;
    font-size: 0.8rem;
    color: var(--bulma-text-weak);
    background: var(--bulma-scheme-main-bis);
    border-bottom: 1px solid var(--bulma-border-weak);
  }

  .transactions-list {
    padding: 0.5rem 0;
  }

  .transaction-item {
    padding: 0.875rem 1.25rem;
    border-bottom: 1px solid var(--bulma-border-weak);
    cursor: pointer;
    transition: background-color 0.15s ease;

    &:hover {
      background: var(--bulma-scheme-main-bis);
    }
  }

  .transaction-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 0.25rem;
  }

  .transaction-date {
    font-size: 0.75rem;
    color: var(--bulma-text-weak);

    i {
      margin-right: 0.35rem;
    }
  }

  .transaction-amount {
    font-weight: 600;
    color: var(--bulma-danger);

    &.negative {
      color: var(--bulma-success);
    }
  }

  .transaction-payee {
    font-weight: 500;
    color: var(--bulma-text-strong);
    margin-bottom: 0.15rem;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .transaction-account {
    font-size: 0.8rem;
    color: var(--bulma-text-weak);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
</style>
