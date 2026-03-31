<script lang="ts">
  import COLORS from "$lib/colors";
  import {
    ajax,
    formatCurrency,
    formatFloat,
    isMobile,
    type AllocationTarget,
    type Forecast,
    type Point,
    type Posting,
    type AssetBreakdown,
    type Legend
  } from "$lib/utils";
  import { onMount, tick, onDestroy } from "svelte";
  import ARIMAPromise from "arima/async";
  import {
    forecast,
    renderProgress,
    findBreakPoints,
    project,
    solvePMTOrNper,
    renderInvestmentTimeline
  } from "$lib/goals";
  import {
    renderAllocationTarget,
    renderBalanceCategoryMap,
    renderCompositionAreaChart,
    computeGoalAllocationTimeline
  } from "$lib/allocation";
  import { generateColorScheme } from "$lib/colors";
  import LegendCard from "$lib/components/LegendCard.svelte";
  import _ from "lodash";
  import LevelItem from "$lib/components/LevelItem.svelte";
  import type { PageData } from "./$types";
  import { iconGlyph } from "$lib/icon";
  import PostingsDrawer from "$lib/components/PostingsDrawer.svelte";
  import dayjs from "dayjs";
  import ProgressWithBreakpoints from "$lib/components/ProgressWithBreakpoints.svelte";
  import AssetsBalance from "$lib/components/AssetsBalance.svelte";
  import BoxLabel from "$lib/components/BoxLabel.svelte";

  export let data: PageData;

  let svg: Element;
  let investmentTimelineSvg: Element;
  let allocationCompositionSvg: Element;
  let targetDateObject: dayjs.Dayjs;
  let savingsTotal = 0,
    investmentTotal = 0,
    gainTotal = 0,
    targetSavings = 0,
    pmt = 0,
    xirr = 0,
    rate = 0,
    paymentPerPeriod = 0,
    targetDate = "",
    name = "",
    icon = "",
    progressPercent = 0,
    breakPoints: Point[] = [],
    savingsTimeline: Point[] = [],
    postings: Posting[] = [],
    latestPostings: Posting[] = [],
    balances: Record<string, AssetBreakdown> = {},
    allocationTargets: AllocationTarget[] = [],
    allocationCompositionLegends: Legend[] = [],
    destroyCallback = () => {},
    balanceDepth = 2;

  let showTransactionsDrawer = false;

  onDestroy(async () => {
    destroyCallback();
  });

  onMount(async () => {
    ({
      savingsTotal,
      investmentTotal,
      gainTotal,
      savingsTimeline,
      target: targetSavings,
      rate,
      targetDate,
      postings,
      icon,
      name,
      xirr,
      paymentPerPeriod,
      balances,
      allocation_targets: allocationTargets
    } = await ajax("/api/goals/savings/:name", null, data));

    latestPostings = _.chain(postings)
      .sortBy((p) => p.date)
      .reverse()
      .take(100)
      .value();

    if (targetSavings != 0) {
      progressPercent = (savingsTotal / targetSavings) * 100;
    }

    ({ pmt, targetDate } = solvePMTOrNper(
      targetSavings,
      rate,
      savingsTotal,
      paymentPerPeriod,
      targetDate
    ));

    let predictionsTimeline: Forecast[] = [];
    targetDateObject = dayjs(targetDate, "YYYY-MM-DD", true);
    if (targetDateObject.isValid()) {
      predictionsTimeline = project(targetSavings, rate, targetDateObject, pmt, savingsTotal);
    } else if (savingsTotal < targetSavings) {
      const ARIMA = await ARIMAPromise;
      predictionsTimeline = forecast(savingsTimeline, targetSavings, ARIMA);
    }

    await tick();
    breakPoints = findBreakPoints(savingsTimeline.concat(predictionsTimeline), targetSavings);
    destroyCallback = renderProgress(savingsTimeline, predictionsTimeline, breakPoints, svg, {
      targetSavings
    });

    await tick();
    if (investmentTimelineSvg) {
      renderInvestmentTimeline(postings, investmentTimelineSvg, pmt);
    }

    const allocationTimeline = computeGoalAllocationTimeline(postings, allocationTargets);
    if (allocationCompositionSvg) {
      allocationCompositionLegends = renderCompositionAreaChart(
        allocationTimeline,
        allocationCompositionSvg
      );
    }

    balanceDepth = renderBalanceCategoryMap(balances, "d3-goal-balance-category") || 2;

    if (allocationTargets?.length > 0) {
      const color = generateColorScheme(allocationTargets.map((t) => t.name));
      renderAllocationTarget(allocationTargets, color, "d3-goal-allocation-target");
      for (const parent of allocationTargets.filter((t) => t.children?.length > 0)) {
        renderAllocationTarget(parent.children, color, `d3-goal-allocation-${parent.name}`);
      }
    }
  });
</script>

<section class="section">
  <div class="container is-fluid">
    <div
      class="box is-clickable transactions-tile"
      on:click={() => (showTransactionsDrawer = true)}
      role="button"
      tabindex="0"
      on:keydown={(e) => e.key === "Enter" && (showTransactionsDrawer = true)}
    >
      <div class="is-flex is-align-items-center" style="gap: 0.75rem;">
        <span class="icon has-text-primary">
          <i class="fas fa-list"></i>
        </span>
        <span class="has-text-weight-semibold">Transactions</span>
        <span class="tag is-light ml-auto">{latestPostings.length}</span>
        <span class="icon has-text-grey">
          <i class="fas fa-chevron-right"></i>
        </span>
      </div>
    </div>
  </div>
</section>

<section class="section">
  <div class="container is-fluid">
    <nav class="level custom-icon {isMobile() && 'grid-2'}">
      <LevelItem title={name} value={iconGlyph(icon)} />
      <LevelItem
        title="Net Investment"
        value={formatCurrency(investmentTotal)}
        color={COLORS.secondary}
        subtitle={`<b>${formatCurrency(gainTotal)}</b> ${gainTotal >= 0 ? "gain" : "loss"}`}
      />

      <LevelItem
        title="Current Savings"
        value={formatCurrency(savingsTotal)}
        color={COLORS.gainText}
        subtitle={`<b>${formatFloat(xirr)}</b> XIRR`}
      />

      <LevelItem
        title="Target Savings"
        value={formatCurrency(targetSavings)}
        color={COLORS.primary}
        subtitle={targetDateObject?.isValid() ? targetDateObject.format("DD MMM YYYY") : null}
      />

      {#if pmt > 0}
        <LevelItem
          title="Monthly Investment needed"
          value={formatCurrency(pmt)}
          color={COLORS.secondary}
          subtitle={rate > 0 ? `Expected <b>${formatFloat(rate, 2)}</b> rate of return` : null}
        />
      {/if}
    </nav>
  </div>
</section>

<section class="section">
  <div class="container is-fluid">
    <ProgressWithBreakpoints {progressPercent} {breakPoints} />
  </div>
</section>

<section class="section tab-retirement-progress">
  <div class="container is-fluid">
    <div class="columns">
      <div class="column is-12">
        <div class="columns flex-wrap">
          <div class="column is-12">
            <div class="box overflow-x-auto">
              <svg height="400" bind:this={svg} />
            </div>
          </div>
        </div>
        <BoxLabel text="{iconGlyph(icon)} {name} progress" />
        <div class="columns">
          <div class="column is-12">
            <div class="box overflow-x-auto">
              <svg height="300" width="100%" bind:this={investmentTimelineSvg} />
            </div>
          </div>
        </div>
        <BoxLabel text="Monthly Investment" />
        <div class="columns">
          <div class="column is-12 has-text-grey">
            <AssetsBalance breakdowns={balances} indent={false} />
          </div>
        </div>
        <BoxLabel text="Current Balance" />
        <div class="columns">
          <div class="column is-12 has-text-centered">
            <div
              id="d3-goal-balance-category"
              style="width: 100%;"
            />
          </div>
        </div>
        <BoxLabel text="Allocation by Category" />
        <div class="columns">
          <div class="column is-12">
            <div class="box overflow-x-auto">
              <LegendCard legends={allocationCompositionLegends} clazz="ml-4" />
              <svg height="300" width="100%" bind:this={allocationCompositionSvg} />
            </div>
          </div>
        </div>
        <BoxLabel text="Asset Allocation" />
        {#if allocationTargets?.length > 0}
          <div class="columns">
            <div class="column is-12">
              <div class="box overflow-x-auto">
                <svg id="d3-goal-allocation-target" width="100%" />
              </div>
            </div>
          </div>
          <BoxLabel text="Allocation" />
          {#each allocationTargets.filter((t) => t.children?.length > 0) as parent}
            <div class="columns">
              <div class="column is-12">
                <div class="box overflow-x-auto">
                  <svg id="d3-goal-allocation-{parent.name}" width="100%" />
                </div>
              </div>
            </div>
            <BoxLabel text="{parent.name} Allocation" />
          {/each}
        {/if}
      </div>
    </div>
  </div>
</section>

<PostingsDrawer
  bind:isOpen={showTransactionsDrawer}
  postings={latestPostings}
  title="Transactions — {name}"
/>
