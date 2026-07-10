<script lang="ts">
  import Table from "$lib/components/Table.svelte";
  import { indendedExpenseAccountName, nonZeroCurrency } from "$lib/table_formatters";
  import { ajax, buildTree, type ExpenseBreakdown } from "$lib/utils";
  import _ from "lodash";
  import { onMount } from "svelte";
  import type { ColumnDefinition } from "tabulator-tables";

  let breakdowns: ExpenseBreakdown[] = [];
  let isEmpty = false;

  onMount(async () => {
    ({ expense_breakdowns: breakdowns } = await ajax("/api/expense/balance"));

    if (_.isEmpty(breakdowns)) {
      isEmpty = true;
    }
  });

  const columns: ColumnDefinition[] = [
    {
      title: "Account",
      field: "group",
      formatter: indendedExpenseAccountName,
      frozen: true
    },
    {
      title: "Amount",
      field: "amount",
      hozAlign: "right",
      formatter: nonZeroCurrency
    }
  ];

  let tree: ExpenseBreakdown[] = [];
  $: if (breakdowns) {
    tree = buildTree(Object.values(breakdowns), (i) => i.group);
  }
</script>

<section class="section" class:is-hidden={!isEmpty}>
  <div class="container is-fluid">
    <div class="columns is-centered">
      <div class="column is-4 has-text-centered">
        <article class="message">
          <div class="message-body">
            <strong>Hurray!</strong> You have no expenses.
          </div>
        </article>
      </div>
    </div>
  </div>
</section>

<section class="section pb-0" class:is-hidden={isEmpty}>
  <div class="container is-fluid">
    <div class="columns">
      <div class="column is-12 pb-0">
        <Table data={tree} tree {columns} />
      </div>
    </div>
  </div>
</section>
