<script lang="ts">
  import { formatCurrency, formatFloat, type CapitalGain, type FYCapitalGain } from "$lib/utils";
  import _ from "lodash";
  import dayjs from "dayjs";
  import CapitalGainDetailCard from "./CapitalGainDetailCard.svelte";
  import Toggleable from "./Toggleable.svelte";

  export let financialYear: string;
  export let capitalGains: CapitalGain[];

  const fyGains: FYCapitalGain[] = _.flatMap(capitalGains, (cg) => cg.fy[financialYear] || []);

  const total = {
    withdrawn: _.sumBy(fyGains, (fy) => fy.sell_price),
    gain: _.sumBy(fyGains, (fy) => fy.tax.gain),
    taxableGain: _.sumBy(fyGains, (fy) => fy.tax.taxable),
    shortTermTaxableGain: _.sumBy(fyGains, (fy) => fy.tax.short_term_taxable),
    longTermTaxableGain: _.sumBy(fyGains, (fy) => fy.tax.long_term_taxable),
    shortTermTax: _.sumBy(fyGains, (fy) => fy.tax.short_term),
    longTermTax: _.sumBy(fyGains, (fy) => fy.tax.long_term),
    slab: _.sumBy(fyGains, (fy) => fy.tax.slab)
  };

  // ITR Schedule CG reports capital gains in these five fixed periods, used
  // to compute quarterly advance tax liability (Section 234C).
  function itrPeriods(financialYear: string) {
    const startYear = parseInt(financialYear.split("-")[0]);
    const endYear = startYear + 1;
    return [
      {
        label: `Up to 15 Jun ${startYear}`,
        start: dayjs(`${startYear}-04-01`),
        end: dayjs(`${startYear}-06-15`)
      },
      {
        label: `16 Jun - 15 Sep ${startYear}`,
        start: dayjs(`${startYear}-06-16`),
        end: dayjs(`${startYear}-09-15`)
      },
      {
        label: `16 Sep - 15 Dec ${startYear}`,
        start: dayjs(`${startYear}-09-16`),
        end: dayjs(`${startYear}-12-15`)
      },
      {
        label: `16 Dec ${startYear} - 15 Mar ${endYear}`,
        start: dayjs(`${startYear}-12-16`),
        end: dayjs(`${endYear}-03-15`)
      },
      {
        label: `16 Mar - 31 Mar ${endYear}`,
        start: dayjs(`${endYear}-03-16`),
        end: dayjs(`${endYear}-03-31`)
      }
    ];
  }

  const allPostingPairs = _.flatMap(fyGains, (fy) => fy.posting_pairs);

  const quarterBreakup = itrPeriods(financialYear).map((period) => {
    const pairs = allPostingPairs.filter(
      (pp) =>
        pp.sell.date.isSameOrAfter(period.start, "day") &&
        pp.sell.date.isSameOrBefore(period.end, "day")
    );
    return {
      label: period.label,
      taxableGain: _.sumBy(pairs, (pp) => pp.tax.taxable),
      shortTermTaxableGain: _.sumBy(pairs, (pp) => pp.tax.short_term_taxable),
      longTermTaxableGain: _.sumBy(pairs, (pp) => pp.tax.long_term_taxable),
      shortTermTax: _.sumBy(pairs, (pp) => pp.tax.short_term),
      longTermTax: _.sumBy(pairs, (pp) => pp.tax.long_term),
      slab: _.sumBy(pairs, (pp) => pp.tax.slab)
    };
  });
</script>

<div class="column is-12">
  <div class="card">
    <header class="card-header">
      <p class="card-header-title">{financialYear}</p>
    </header>

    <div class="card-content">
      <div class="content">
        <div class="columns">
          <div class="column is-4">
            <table class="table is-narrow is-fullwidth is-hoverable">
              <tbody>
                <tr>
                  <td>Withdrawn</td>
                  <td class="has-text-right has-text-weight-bold"
                    >{formatCurrency(total["withdrawn"], 2)}</td
                  >
                </tr>
                <tr>
                  <td>Gain</td>
                  <td class="has-text-right has-text-weight-bold"
                    >{formatCurrency(total["gain"], 2)}</td
                  >
                </tr>
                <tr>
                  <td>Taxable Gain</td>
                  <td class="has-text-right has-text-weight-bold"
                    >{formatCurrency(total["taxableGain"], 2)}</td
                  >
                </tr>
                <tr>
                  <td>Short Term Taxable Gain</td>
                  <td class="has-text-right has-text-weight-bold"
                    >{formatCurrency(total["shortTermTaxableGain"], 2)}</td
                  >
                </tr>
                <tr>
                  <td>Long Term Taxable Gain</td>
                  <td class="has-text-right has-text-weight-bold"
                    >{formatCurrency(total["longTermTaxableGain"], 2)}</td
                  >
                </tr>
                <tr>
                  <td>Short Term Tax</td>
                  <td class="has-text-right has-text-weight-bold"
                    >{formatCurrency(total["shortTermTax"], 2)}</td
                  >
                </tr>
                <tr>
                  <td>Long Term Tax</td>
                  <td class="has-text-right has-text-weight-bold"
                    >{formatCurrency(total["longTermTax"], 2)}</td
                  >
                </tr>
                <tr>
                  <td>Taxable at Slab Rate</td>
                  <td class="has-text-right has-text-weight-bold"
                    >{formatCurrency(total["slab"], 2)}</td
                  >
                </tr>
              </tbody>
            </table>

            <Toggleable>
              <a slot="toggle" let:active let:onclick on:click={(e) => onclick(e)}>
                <span class="icon has-text-link">
                  <i
                    class="fas {active ? 'fa-chevron-up' : 'fa-chevron-down'}"
                    aria-hidden="true"
                  />
                </span>
                Quarter-wise Breakup
              </a>
              <div slot="content" class="overflow-x-auto mt-3">
                <table class="table is-narrow is-fullwidth is-hoverable is-size-7">
                  <thead>
                    <tr>
                      <th>Period</th>
                      <th class="has-text-right">Taxable Gain</th>
                      <th class="has-text-right">ST Taxable Gain</th>
                      <th class="has-text-right">LT Taxable Gain</th>
                      <th class="has-text-right">Short Term Tax</th>
                      <th class="has-text-right">Long Term Tax</th>
                      <th class="has-text-right">Slab Rate</th>
                    </tr>
                  </thead>
                  <tbody>
                    {#each quarterBreakup as q}
                      <tr>
                        <td>{q.label}</td>
                        <td class="has-text-right">{formatCurrency(q.taxableGain, 2)}</td>
                        <td class="has-text-right">{formatCurrency(q.shortTermTaxableGain, 2)}</td>
                        <td class="has-text-right">{formatCurrency(q.longTermTaxableGain, 2)}</td>
                        <td class="has-text-right">{formatCurrency(q.shortTermTax, 2)}</td>
                        <td class="has-text-right">{formatCurrency(q.longTermTax, 2)}</td>
                        <td class="has-text-right">{formatCurrency(q.slab, 2)}</td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              </div>
            </Toggleable>
          </div>
          <div class="column is-8 overflow-x-auto">
            <table class="table is-narrow is-fullwidth is-hoverable">
              <thead>
                <tr>
                  <th />
                  <th>Account</th>
                  <th>Tax Category</th>
                  <th class="has-text-right">Sold Units</th>
                  <th class="has-text-right">Purchase Price</th>
                  <th class="has-text-right">Average Purchase Unit Price</th>
                  <th class="has-text-right">Sell Price</th>
                  <th class="has-text-right">Average Sell Unit Price</th>
                  <th class="has-text-right">Gain</th>
                  <th class="has-text-right">Taxable Gain</th>
                  <th class="has-text-right">Short Term Taxable Gain</th>
                  <th class="has-text-right">Long Term Taxable Gain</th>
                  <th class="has-text-right">Short Term Tax</th>
                  <th class="has-text-right">Long Term Tax</th>
                  <th class="has-text-right">Taxable at Slat Rate</th>
                </tr>
              </thead>
              <tbody>
                {#each capitalGains as cg}
                  {#if cg.fy[financialYear]}
                    {@const fy = cg.fy[financialYear]}
                    <Toggleable>
                      <tr
                        class={active ? "is-active has-background-white-ter" : ""}
                        style="cursor: pointer;"
                        slot="toggle"
                        let:active
                        let:onclick
                        on:click={(e) => onclick(e)}
                      >
                        <td>
                          <span class="icon has-text-link">
                            <i
                              class="fas {active ? 'fa-chevron-up' : 'fa-chevron-down'}"
                              aria-hidden="true"
                            />
                          </span>
                        </td>
                        <td>{cg.account}</td>
                        <td>{cg.tax_category}</td>
                        <td class="has-text-right">{formatFloat(fy.units)}</td>
                        <td class="has-text-right">{formatCurrency(fy.purchase_price, 2)}</td>
                        <td class="has-text-right"
                          >{formatCurrency(fy.purchase_price / fy.units, 4)}</td
                        >
                        <td class="has-text-right">{formatCurrency(fy.sell_price, 2)}</td>
                        <td class="has-text-right">{formatCurrency(fy.sell_price / fy.units, 4)}</td
                        >
                        <td class="has-text-right has-text-weight-bold"
                          >{formatCurrency(fy.tax.gain, 2)}</td
                        >
                        <td class="has-text-right has-text-weight-bold"
                          >{formatCurrency(fy.tax.taxable, 2)}</td
                        >
                        <td class="has-text-right has-text-weight-bold"
                          >{formatCurrency(fy.tax.short_term_taxable, 2)}</td
                        >
                        <td class="has-text-right has-text-weight-bold"
                          >{formatCurrency(fy.tax.long_term_taxable, 2)}</td
                        >
                        <td class="has-text-right has-text-weight-bold"
                          >{formatCurrency(fy.tax.short_term, 2)}</td
                        >
                        <td class="has-text-right has-text-weight-bold"
                          >{formatCurrency(fy.tax.long_term, 2)}</td
                        >
                        <td class="has-text-right has-text-weight-bold"
                          >{formatCurrency(fy.tax.slab, 2)}</td
                        >
                      </tr>
                      <tr slot="content">
                        <td colspan="15" class="p-0">
                          <CapitalGainDetailCard fyCapitalGain={fy} />
                        </td>
                      </tr>
                    </Toggleable>
                  {/if}
                {/each}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
  </div>
</div>
