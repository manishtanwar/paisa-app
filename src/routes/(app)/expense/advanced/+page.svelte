<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import _ from "lodash";
  import {
    ajax,
    secondName,
    type Posting,
    formatCurrency,
    formatPercentage,
    type Legend,
    parentName,
    lastName,
    tooltip,
    formatFloat,
    darkenOrLighten
  } from "$lib/utils";
  import { month, setAllowedDateRange } from "../../../../store";
  import { writable } from "svelte/store";
  import COLORS, { generateColorScheme } from "$lib/colors";
  import ZeroState from "$lib/components/ZeroState.svelte";
  import BoxLabel from "$lib/components/BoxLabel.svelte";
  import LegendCard from "$lib/components/LegendCard.svelte";
  import ContextMenu from "$lib/components/ContextMenu.svelte";
  import TransactionsDrawer from "$lib/components/TransactionsDrawer.svelte";
  import HierarchicalBreadcrumb from "$lib/components/HierarchicalBreadcrumb.svelte";
  import * as d3 from "d3";

  let expenses: Posting[] = [];
  let grouped_expenses: Record<string, Posting[]> = {};
  let current_month_expenses: Posting[] = [];
  let expense_aggregates: Record<string, any> = {};
  let total = 0;
  let legends: Legend[] = [];
  let color: d3.ScaleOrdinal<string, string>;
  let depth = 2;

  // Timeline section variables
  let timelineData: any[] = [];
  let selectedPeriod = "6m";
  let timelineTotal = 0;
  let selectedAccount: string | null = null;

  // TreeMap filtering variables
  let filteredExpenseAggregates: Record<string, any> = {};
  let isFilteredView = false;
  let originalExpenseAggregates: Record<string, any> = {};

  // Context Menu variables
  let showContextMenu = false;
  let contextMenuX = 0;
  let contextMenuY = 0;
  let contextMenuAccount = "";
  let contextMenuHasChildren = true;

  // Transactions Drawer variables
  let showDrawer = false;
  let drawerAccount = "";

  // Deep-dive view (leaf account)
  let isLeafAccount = false;
  let leafTransactions: any[] = [];

  $: {
    current_month_expenses = _.chain((grouped_expenses && grouped_expenses[$month]) || [])
      .filter((e) => e.amount > 0)
      .sortBy((e) => e.date)
      .reverse()
      .value();
  }

  $: if (grouped_expenses && Object.keys(grouped_expenses).length > 0 && $month) {
    // Get hierarchical aggregates for the selected month
    loadExpenseAllocation();
  }

  $: if (isFilteredView && selectedAccount && Object.keys(expense_aggregates).length > 0) {
    // Render filtered TreeMap when account is selected
    renderFilteredExpenseTreeMap();
  } else if (!isFilteredView && Object.keys(expense_aggregates).length > 0) {
    // Render original TreeMap when not filtered
    renderExpenseTreeMap();
  }

  $: if (selectedPeriod || selectedAccount !== null) {
    // Load timeline data when period or account changes
    loadExpenseTimeline();
  }

  $: if (timelineData && timelineData.length > 0) {
    // Re-render timeline when data changes
    renderExpenseTimeline();
  }

  $: if (selectedAccount !== null) {
    // Highlight selected account when it changes
    highlightSelectedAccount();
  }

  async function loadExpenseAllocation() {
    try {
      console.log("Loading expense allocation for month:", $month);
      // Pass the current month as a query parameter
      const data = await ajax(`/api/expense/allocation?month=${$month}`);
      console.log("API Response:", data);
      console.log("Expense aggregates:", data.aggregates);

      expense_aggregates = data.aggregates;
      originalExpenseAggregates = { ...data.aggregates }; // Store original for reset

      total = _.sumBy(_.values(expense_aggregates), (a) => a.amount);
      console.log("Total:", total);

      // Generate color scheme
      const accounts = _.keys(expense_aggregates);
      console.log("Accounts:", accounts);
      color = generateColorScheme(accounts);

      // Calculate depth for dynamic height
      depth = _.max(_.map(accounts, (account) => account.split(":").length)) || 2;
      console.log("Depth:", depth);

      // Reset filtered view when loading new data
      isFilteredView = false;
      selectedAccount = null;
    } catch (error) {
      console.error("Error loading expense allocation:", error);
    }
  }

  async function loadExpenseTimeline() {
    try {
      console.log(
        "Loading expense timeline for period:",
        selectedPeriod,
        "account:",
        selectedAccount
      );
      let url = `/api/expense/timeline?period=${selectedPeriod}`;
      if (selectedAccount) {
        url += `&account=${encodeURIComponent(selectedAccount)}`;
      }
      const data = await ajax(url);
      console.log("Timeline data:", data);

      timelineData = data.timeline;
      timelineTotal = _.sumBy(timelineData, (d) => d.amount);

      console.log("Timeline data set:", timelineData);
      console.log("Timeline total:", timelineTotal);
    } catch (error) {
      console.error("Error loading expense timeline:", error);
    }
  }

  function selectAccount(account: string) {
    console.log("Selecting account:", account);
    selectedAccount = account;
    isFilteredView = true;

    // Filter aggregates to show only selected account and its children
    filterExpenseAggregates(account);
  }

  function clearAccountSelection() {
    console.log("Clearing account selection");
    selectedAccount = null;
    isFilteredView = false;
    isLeafAccount = false;
    showContextMenu = false;

    // Reset to original aggregates
    expense_aggregates = { ...originalExpenseAggregates };
    total = _.sumBy(_.values(expense_aggregates), (a) => a.amount);

    // Update color scheme for all accounts
    const accounts = _.keys(expense_aggregates);
    color = generateColorScheme(accounts);

    // Reset depth
    depth = _.max(_.map(accounts, (account) => account.split(":").length)) || 2;
  }

  // Context Menu handlers
  function showAccountContextMenu(event: MouseEvent, account: string) {
    event.stopPropagation();
    contextMenuX = event.clientX;
    contextMenuY = event.clientY;
    contextMenuAccount = account;

    // Check if account has children
    contextMenuHasChildren = Object.keys(originalExpenseAggregates).some(
      (key) => key.startsWith(account + ":") && key !== account
    );

    showContextMenu = true;
  }

  function handleContextMenuClose() {
    showContextMenu = false;
  }

  function handleContextMenuZoom(event: CustomEvent<{ account: string }>) {
    const { account } = event.detail;
    selectAccount(account);
    checkIfLeafAccount(account);
  }

  function handleContextMenuViewTransactions(event: CustomEvent<{ account: string }>) {
    const { account } = event.detail;
    drawerAccount = account;
    showDrawer = true;
  }

  function handleContextMenuViewTrend(event: CustomEvent<{ account: string }>) {
    const { account } = event.detail;
    selectedAccount = account;
    isFilteredView = true;
    filterExpenseAggregates(account);

    // Scroll to timeline section
    const timelineEl = document.getElementById("d3-expense-timeline");
    if (timelineEl) {
      timelineEl.scrollIntoView({ behavior: "smooth", block: "start" });
    }
  }

  function handleBreadcrumbNavigate(event: CustomEvent<{ account: string }>) {
    const { account } = event.detail;
    selectAccount(account);
    checkIfLeafAccount(account);
  }

  function handleBreadcrumbNavigateHome() {
    clearAccountSelection();
  }

  function checkIfLeafAccount(account: string) {
    // Check if this account has any children
    const hasChildren = Object.keys(originalExpenseAggregates).some(
      (key) => key.startsWith(account + ":") && key !== account
    );
    isLeafAccount = !hasChildren;
  }

  function handleDrawerClose() {
    showDrawer = false;
  }

  function filterExpenseAggregates(account: string) {
    console.log("Filtering aggregates for account:", account);

    // Filter to include only the selected account and its children
    const filtered: Record<string, any> = {};

    // Add the selected account itself
    if (originalExpenseAggregates[account]) {
      filtered[account] = originalExpenseAggregates[account];
    }

    // Add all child accounts
    Object.keys(originalExpenseAggregates).forEach((key) => {
      if (key.startsWith(account + ":") && key !== account) {
        filtered[key] = originalExpenseAggregates[key];
      }
    });

    // If no children found, just show the selected account
    if (Object.keys(filtered).length === 0 && originalExpenseAggregates[account]) {
      filtered[account] = originalExpenseAggregates[account];
    }

    console.log("Filtered aggregates:", filtered);

    // Update the aggregates and total
    expense_aggregates = filtered;
    total = _.sumBy(_.values(expense_aggregates), (a) => a.amount);

    // Generate color scheme for all hierarchical levels, not just leaf accounts
    const allHierarchicalAccounts = generateHierarchicalAccounts(account);
    color = generateColorScheme(allHierarchicalAccounts);

    // Update depth for filtered view
    const accounts = _.keys(expense_aggregates);
    depth = _.max(_.map(accounts, (account) => account.split(":").length)) || 2;
  }

  function generateHierarchicalAccounts(selectedAccount: string): string[] {
    const accounts: string[] = [];
    const parts = selectedAccount.split(":");

    // Generate all parent levels
    for (let i = 1; i <= parts.length; i++) {
      const accountPath = parts.slice(0, i).join(":");
      accounts.push(accountPath);
    }

    // Add any child accounts that exist in the original data
    Object.keys(originalExpenseAggregates).forEach((key) => {
      if (key.startsWith(selectedAccount + ":") && key !== selectedAccount) {
        accounts.push(key);
      }
    });

    console.log("Generated hierarchical accounts for coloring:", accounts);
    return accounts;
  }

  function highlightSelectedAccount() {
    // Remove previous highlights
    d3.selectAll(".node").style("border", "1px solid rgba(255,255,255,0.2)");

    // Highlight selected account
    if (selectedAccount) {
      d3.selectAll(".node")
        .filter((d: any) => d.id === selectedAccount)
        .style("border", "3px solid #3273dc")
        .style("box-shadow", "0 0 10px rgba(50, 115, 220, 0.5)");
    }
  }

  function createCompleteHierarchy(aggregates: Record<string, any>): any[] {
    const result: any[] = [];
    const allAccounts = new Set<string>();

    // Add all existing accounts
    _.forEach(aggregates, (aggregate, account) => {
      allAccounts.add(account);
      result.push(aggregate);
    });

    // Add missing parent accounts
    _.forEach(aggregates, (aggregate, account) => {
      const parts = account.split(":");
      for (let i = 1; i < parts.length; i++) {
        const parentAccount = parts.slice(0, i).join(":");
        if (!allAccounts.has(parentAccount)) {
          allAccounts.add(parentAccount);
          result.push({
            account: parentAccount,
            amount: 0 // D3 will calculate this from children
          });
        }
      }
    });

    return result;
  }

  onMount(async () => {
    const data = await ajax("/api/expense");
    expenses = data.expenses;
    grouped_expenses = data.month_wise.expenses;

    setAllowedDateRange(_.map(expenses, (e) => e.date));

    // Load timeline data on initial mount
    loadExpenseTimeline();
  });

  function renderExpenseTreeMap() {
    console.log("renderExpenseTreeMap called with aggregates:", expense_aggregates);
    if (_.isEmpty(expense_aggregates)) {
      console.log("No aggregates to render");
      return;
    }

    // Add transition effect when returning to original view
    const container = d3.select("#d3-expense-treemap");
    container.style("opacity", 0);

    // Clear previous content
    container.selectAll("*").remove();

    // Render with transition
    setTimeout(() => {
      container.transition().duration(300).style("opacity", 1);
      renderTreeMapContent();
    }, 50);
  }

  function renderFilteredExpenseTreeMap() {
    console.log("renderFilteredExpenseTreeMap called with aggregates:", expense_aggregates);
    if (_.isEmpty(expense_aggregates)) {
      console.log("No filtered aggregates to render");
      return;
    }

    // Add transition effect
    const container = d3.select("#d3-expense-treemap");
    container.style("opacity", 0);

    // Clear previous content
    container.selectAll("*").remove();

    // Render with transition
    setTimeout(() => {
      container.transition().duration(300).style("opacity", 1);
      renderTreeMapContent();
    }, 50);
  }

  function renderTreeMapContent() {
    if (_.isEmpty(expense_aggregates)) {
      console.log("No aggregates to render");
      return;
    }

    const div = d3.select("#d3-expense-treemap");
    const margin = { top: 0, right: 20, bottom: 0, left: 0 };
    const width = div.node().parentElement.clientWidth - margin.left - margin.right;
    const height = +div.style("height").replace("px", "") - margin.top - margin.bottom;

    // Create complete hierarchy by adding missing parent accounts
    const completeHierarchy = createCompleteHierarchy(expense_aggregates);
    console.log("Complete hierarchy:", completeHierarchy);

    const stratify = d3
      .stratify<any>()
      .id((d) => d.account)
      .parentId((d) => {
        const parent = parentName(d.account);
        return parent && parent !== d.account ? parent : null;
      });

    const partition = d3.partition().size([width, height]).round(true);

    let root;
    try {
      root = stratify(_.sortBy(completeHierarchy, (a) => a.account))
        .sum((a) => a.amount)
        .sort(function (a, b) {
          return b.height - a.height || b.value - a.value;
        });
    } catch (error) {
      console.error("Error creating hierarchy:", error);
      return;
    }

    partition(root);

    const percent = (d: d3.HierarchyNode<any>) => {
      return formatFloat((d.value / root.value) * 100) + "%";
    };

    const cell = div
      .selectAll(".node")
      .data(root.descendants())
      .enter()
      .append("div")
      .attr("class", "node")
      .attr("data-tippy-content", (d) => {
        return tooltip([
          ["Account", [d.id, "has-text-right"]],
          ["Amount", [formatCurrency(d.value), "has-text-weight-bold has-text-right"]],
          ["Percentage", [percent(d), "has-text-weight-bold has-text-right"]]
        ]);
      })
      .style("position", "absolute")
      .style("top", (d: any) => d.y0 + "px")
      .style("left", (d: any) => d.x0 + "px")
      .style("width", (d: any) => d.x1 - d.x0 + "px")
      .style("height", (d: any) => d.y1 - d.y0 + "px")
      .style("background", (d) => color(d.id))
      .style("color", (d) => darkenOrLighten(color(d.id)))
      .style("border", "1px solid rgba(255,255,255,0.2)")
      .style("box-sizing", "border-box")
      .style("cursor", "pointer")
      .on("click", function (event, d) {
        console.log("TreeMap cell clicked:", d.id);
        showAccountContextMenu(event, d.id);
      });

    cell
      .append("p")
      .attr("class", "heading has-text-weight-bold")
      .style("margin", "4px")
      .style("font-size", "0.8rem")
      .text((d) => lastName(d.id));

    cell
      .append("p")
      .attr("class", "heading has-text-weight-bold")
      .style("margin", "4px")
      .style("font-size", "0.6rem")
      .text(percent);

    cell
      .append("p")
      .attr("class", "heading has-text-weight-bold")
      .style("margin", "4px")
      .style("font-size", "0.6rem")
      .text((d) => formatCurrency(d.value));

    // Generate legends
    legends = _.map(_.keys(expense_aggregates), (account) => ({
      label: secondName(account),
      color: color(account),
      shape: "square"
    }));
  }

  function renderExpenseTimeline() {
    console.log("renderExpenseTimeline called with data:", timelineData);
    if (_.isEmpty(timelineData)) {
      console.log("No timeline data to render");
      return;
    }

    // Check if the timeline container exists
    const timelineContainer = document.getElementById("d3-expense-timeline");
    if (!timelineContainer) {
      console.log("Timeline container not found");
      return;
    }

    // Clear previous content
    d3.select("#d3-expense-timeline").selectAll("*").remove();

    const div = d3.select("#d3-expense-timeline");
    const margin = { top: 20, right: 30, bottom: 40, left: 60 };
    const width = div.node().parentElement.clientWidth - margin.left - margin.right;
    const height = 300 - margin.top - margin.bottom;

    try {
      // Create SVG
      const svg = div
        .append("svg")
        .attr("width", width + margin.left + margin.right)
        .attr("height", height + margin.top + margin.bottom);

      const g = svg.append("g").attr("transform", `translate(${margin.left},${margin.top})`);

      // Create scales
      console.log("Creating scales with data:", timelineData);

      // Format month labels for display
      const formatMonthLabel = (monthStr: string) => {
        const [year, month] = monthStr.split("-");
        const date = new Date(parseInt(year), parseInt(month) - 1);
        return d3.timeFormat("%b %y")(date);
      };

      const xScale = d3
        .scaleBand()
        .domain(timelineData.map((d) => d.month))
        .range([0, width])
        .padding(0.1);

      const yScale = d3
        .scaleLinear()
        .domain([0, d3.max(timelineData, (d) => d.amount) || 0])
        .range([height, 0]);

      console.log("X scale domain:", xScale.domain());
      console.log("Y scale domain:", yScale.domain());

      // Create line generator
      const line = d3
        .line<any>()
        .x((d) => xScale(d.month)! + xScale.bandwidth() / 2)
        .y((d) => yScale(d.amount))
        .curve(d3.curveMonotoneX);

      console.log("Line generator created");

      // Add the line
      g.append("path")
        .datum(timelineData)
        .attr("fill", "none")
        .attr("stroke", COLORS.expenses)
        .attr("stroke-width", 2)
        .attr("d", line);

      console.log("Line added to chart");

      // Add circles for data points
      g.selectAll(".dot")
        .data(timelineData)
        .enter()
        .append("circle")
        .attr("class", "dot")
        .attr("cx", (d) => xScale(d.month)! + xScale.bandwidth() / 2)
        .attr("cy", (d) => yScale(d.amount))
        .attr("r", 4)
        .attr("fill", COLORS.expenses)
        .attr("stroke", "white")
        .attr("stroke-width", 2);

      // Add X axis
      g.append("g")
        .attr("transform", `translate(0,${height})`)
        .call(d3.axisBottom(xScale).tickFormat((d) => formatMonthLabel(d as string)))
        .selectAll("text")
        .style("text-anchor", "end")
        .attr("dx", "-.8em")
        .attr("dy", ".15em")
        .attr("transform", "rotate(-45)");

      // Add Y axis
      g.append("g").call(d3.axisLeft(yScale).tickFormat((d) => formatCurrency(d as number)));

      // Calculate and add average line
      const average = d3.mean(timelineData, (d) => d.amount) || 0;
      console.log("Average value:", average);

      // Add average line background highlight
      g.append("rect")
        .attr("x", 0)
        .attr("y", yScale(average) - 1)
        .attr("width", width)
        .attr("height", 2)
        .attr("fill", "#4a90e2")
        .style("opacity", 0.1);

      // Add average line
      g.append("line")
        .attr("x1", 0)
        .attr("x2", width)
        .attr("y1", yScale(average))
        .attr("y2", yScale(average))
        .attr("stroke", "#4a90e2")
        .attr("stroke-width", 2)
        .attr("stroke-dasharray", "5,5")
        .style("opacity", 0.9)
        .style("cursor", "pointer");

      // Add average label
      g.append("text")
        .attr("x", width - 10)
        .attr("y", yScale(average) - 10)
        .attr("text-anchor", "end")
        .attr("font-size", "12px")
        .attr("font-weight", "bold")
        .attr("fill", "#4a90e2")
        .text(`Avg: ${formatCurrency(average)}`);

      // Add tooltips
      const tooltip = d3
        .select("body")
        .append("div")
        .attr("class", "tooltip")
        .style("opacity", 0)
        .style("position", "absolute")
        .style("background", "rgba(0, 0, 0, 0.8)")
        .style("color", "white")
        .style("padding", "8px")
        .style("border-radius", "4px")
        .style("font-size", "12px")
        .style("pointer-events", "none");

      g.selectAll(".dot")
        .on("mouseover", function (event, d) {
          tooltip.transition().duration(200).style("opacity", 0.9);
          tooltip
            .html(
              `
          <div><strong>${formatMonthLabel(d.month)}</strong></div>
          <div>Amount: ${formatCurrency(d.amount)}</div>
        `
            )
            .style("left", event.pageX + 10 + "px")
            .style("top", event.pageY - 28 + "px");
        })
        .on("mouseout", function () {
          tooltip.transition().duration(500).style("opacity", 0);
        });

      // Add tooltip for average line
      g.selectAll("line")
        .filter(function () {
          return d3.select(this).attr("stroke") === "#4a90e2";
        })
        .on("mouseover", function (event) {
          tooltip.transition().duration(200).style("opacity", 0.9);
          tooltip
            .html(
              `
          <div><strong>Average</strong></div>
          <div>Amount: ${formatCurrency(average)}</div>
          <div>Data Points: ${timelineData.length}</div>
        `
            )
            .style("left", event.pageX + 10 + "px")
            .style("top", event.pageY - 28 + "px");
        })
        .on("mouseout", function () {
          tooltip.transition().duration(500).style("opacity", 0);
        });
    } catch (error) {
      console.error("Error rendering timeline chart:", error);
    }
  }
</script>

<section class="section tab-expense">
  <div class="container is-fluid">
    <div class="columns">
      <div class="column is-12 has-text-centered">
        <div class="box overflow-x-auto">
          <div
            id="d3-expense-treemap"
            style="width: 100%; height: {depth * 100}px; position: relative"
          />
        </div>
      </div>
    </div>
    <div class="level">
      <div class="level-left">
        <div class="level-item">
          <BoxLabel
            text="Expenses by Category{isFilteredView
              ? ` - ${selectedAccount}`
              : ''} - {formatCurrency(total)} (Click any category for options)"
          />
          {#if isFilteredView && selectedAccount}
            <HierarchicalBreadcrumb
              account={selectedAccount}
              colorScheme={color}
              on:navigate={handleBreadcrumbNavigate}
              on:navigateHome={handleBreadcrumbNavigateHome}
            />
          {/if}
        </div>
      </div>
      <div class="level-right">
        <div class="level-item">
          {#if isFilteredView}
            <button class="button is-small is-light" on:click={clearAccountSelection}>
              <span class="icon">
                <i class="fas fa-arrow-left"></i>
              </span>
              <span>Show All Categories</span>
            </button>
          {/if}
        </div>
      </div>
    </div>
  </div>
</section>

<!-- Timeline Section -->
<section class="section tab-expense">
  <div class="container is-fluid">
    <div class="columns">
      <div class="column is-12">
        <div class="box">
          <div class="level">
            <div class="level-left">
              <div class="level-item">
                <h2 class="title is-3">
                  {#if selectedAccount}
                    <div class="is-inline-block ml-3">
                      <div
                        class="notification is-primary is-light py-2 px-3"
                        style="display: inline-block; margin: 0;"
                      >
                        <div class="is-flex is-align-items-center">
                          <span class="icon-text">
                            <span class="icon">
                              <i class="fas fa-chart-line"></i>
                            </span>
                            <span class="has-text-weight-bold">{selectedAccount}</span>
                          </span>
                          <button
                            class="delete is-small ml-2"
                            on:click={clearAccountSelection}
                            aria-label="Clear selection"
                          ></button>
                        </div>
                      </div>
                    </div>
                  {/if}
                </h2>
              </div>
            </div>
            <div class="level-right">
              <div class="level-item">
                <div class="field has-addons">
                  <div class="control">
                    <button
                      class="button is-small {selectedPeriod === '3m' ? 'is-primary' : ''}"
                      on:click={() => (selectedPeriod = "3m")}
                    >
                      3M
                    </button>
                  </div>
                  <div class="control">
                    <button
                      class="button is-small {selectedPeriod === '6m' ? 'is-primary' : ''}"
                      on:click={() => (selectedPeriod = "6m")}
                    >
                      6M
                    </button>
                  </div>
                  <div class="control">
                    <button
                      class="button is-small {selectedPeriod === '1y' ? 'is-primary' : ''}"
                      on:click={() => (selectedPeriod = "1y")}
                    >
                      1Y
                    </button>
                  </div>
                  <div class="control">
                    <button
                      class="button is-small {selectedPeriod === '3y' ? 'is-primary' : ''}"
                      on:click={() => (selectedPeriod = "3y")}
                    >
                      3Y
                    </button>
                  </div>
                  {#if selectedAccount}
                    <div class="control">
                      <button class="button is-small is-light" on:click={clearAccountSelection}>
                        Show All
                      </button>
                    </div>
                  {/if}
                </div>
              </div>
            </div>
          </div>
          <div id="d3-expense-timeline" style="width: 100%; height: 300px;"></div>
        </div>
      </div>
    </div>
    <BoxLabel
      text="Total Expenses{selectedAccount ? ` for ${selectedAccount}` : ''}: {formatCurrency(
        timelineTotal
      )}"
    />
  </div>
</section>

{#if current_month_expenses.length === 0}
  <section class="section tab-expense">
    <div class="container is-fluid">
      <div class="columns">
        <div class="column is-12">
          <ZeroState item={current_month_expenses}>
            <strong>No expenses found</strong> for the selected month.
          </ZeroState>
        </div>
      </div>
    </div>
  </section>
{/if}

<!-- Context Menu -->
{#if showContextMenu}
  <ContextMenu
    x={contextMenuX}
    y={contextMenuY}
    account={contextMenuAccount}
    hasChildren={contextMenuHasChildren}
    on:close={handleContextMenuClose}
    on:zoom={handleContextMenuZoom}
    on:viewTransactions={handleContextMenuViewTransactions}
    on:viewTrend={handleContextMenuViewTrend}
  />
{/if}

<!-- Transactions Drawer -->
<TransactionsDrawer
  bind:isOpen={showDrawer}
  account={drawerAccount}
  month={$month}
  on:close={handleDrawerClose}
/>
