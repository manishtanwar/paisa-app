import * as d3 from "d3";
import dayjs from "dayjs";
import tippy from "tippy.js";
import _ from "lodash";
import {
  type Posting,
  type Aggregate,
  type AllocationTarget,
  type AssetBreakdown,
  formatCurrency,
  formatFloat,
  lastName,
  parentName,
  secondName,
  tooltip,
  skipTicks,
  rem,
  now,
  type Legend,
  darkenOrLighten
} from "./utils";
import COLORS, { generateColorScheme } from "./colors";
import chroma from "chroma-js";

export function renderAllocationTarget(
  allocationTargets: AllocationTarget[],
  color: d3.ScaleOrdinal<string, string>,
  containerId = "d3-allocation-target"
) {
  const id = `#${containerId}`;

  if (_.isEmpty(allocationTargets)) {
    return;
  }
  allocationTargets = _.sortBy(allocationTargets, (t) => t.name);
  const BAR_HEIGHT = rem(25);
  const svg = d3.select(id),
    margin = { top: rem(20), right: rem(20), bottom: rem(10), left: rem(150) },
    fullWidth = Math.max(document.getElementById(id.substring(1)).parentElement.clientWidth, 1000),
    width = fullWidth - margin.left - margin.right,
    height = allocationTargets.length * BAR_HEIGHT * 2,
    g = svg.append("g").attr("transform", "translate(" + margin.left + "," + margin.top + ")");
  svg.attr("height", height + margin.top + margin.bottom);

  svg.attr("width", fullWidth);

  const keys = ["target", "current"];
  const colorKeys = ["target", "current", "diff"];
  const colors = [COLORS.primary, COLORS.secondary, COLORS.diff];

  const y = d3.scaleBand().range([0, height]).paddingInner(0).paddingOuter(0);
  y.domain(allocationTargets.map((t) => t.name));

  const y1 = d3
    .scaleBand()
    .range([0, y.bandwidth()])
    .domain(keys)
    .paddingInner(0)
    .paddingOuter(0.1);

  const z = d3.scaleOrdinal<string>(colors).domain(colorKeys);

  const z1 = d3
    .scaleThreshold<number, string>()
    .domain([5, 10, 15])
    .range([COLORS.gain, COLORS.warn, COLORS.loss, COLORS.loss]);

  const maxX = _.chain(allocationTargets)
    .flatMap((t) => [t.current, t.target])
    .max()
    .value();
  const targetWidth = rem(400);
  const targetMargin = rem(20);
  const textGroupWidth = rem(150);
  const valueColumnWidth = rem(200);
  const textGroupMargin = rem(20);
  const textGroupZero = targetWidth + targetMargin;

  const x = d3
    .scaleLinear()
    .range([textGroupZero + textGroupWidth + valueColumnWidth + textGroupMargin, width]);
  x.domain([0, maxX]);
  const x1 = d3.scaleLinear().range([0, targetWidth]).domain([0, maxX]);

  g.append("line")
    .classed("svg-grey-lightest", true)
    .attr("x1", 0)
    .attr("y1", height)
    .attr("x2", width)
    .attr("y2", height);

  g.append("text")
    .classed("svg-text-grey", true)
    .text("Target")
    .attr("text-anchor", "end")

    .attr("x", textGroupZero + (textGroupWidth * 1) / 3)
    .attr("y", -5);

  g.append("text")
    .classed("svg-text-grey", true)
    .text("Current")
    .attr("text-anchor", "end")
    .attr("x", textGroupZero + (textGroupWidth * 2) / 3)
    .attr("y", -5);

  g.append("text")
    .classed("svg-text-grey", true)
    .text("Diff")
    .attr("text-anchor", "end")
    .attr("x", textGroupZero + textGroupWidth)
    .attr("y", -5);

  g.append("text")
    .classed("svg-text-grey", true)
    .text("Current Value")
    .attr("text-anchor", "end")
    .attr("x", textGroupZero + textGroupWidth + valueColumnWidth / 2)
    .attr("y", -5);

  g.append("text")
    .classed("svg-text-grey", true)
    .text("Target Value")
    .attr("text-anchor", "end")
    .attr("x", textGroupZero + textGroupWidth + valueColumnWidth)
    .attr("y", -5);

  g.append("g")
    .attr("class", "axis y")
    .attr("transform", "translate(0," + height + ")")
    .call(
      d3
        .axisBottom(x1)
        .tickSize(-height)
        .tickFormat(skipTicks(40, x, (n: number) => formatFloat(n, 0)))
    );

  g.append("g").attr("class", "axis y dark").call(d3.axisLeft(y));

  const textGroup = g
    .append("g")
    .selectAll("g")
    .data(allocationTargets)
    .enter()
    .append("g")
    .attr("class", "inline-text");

  textGroup
    .append("line")
    .classed("svg-grey-lightest", true)
    .attr("x1", 0)
    .attr("y1", (t) => y(t.name))
    .attr("x2", width)
    .attr("y2", (t) => y(t.name));

  textGroup
    .append("text")
    .text((t) => formatFloat(t.target))
    .attr("text-anchor", "end")
    .attr("dominant-baseline", "middle")
    .style("fill", z("target"))
    .attr("x", textGroupZero + (textGroupWidth * 1) / 3)
    .attr("y", (t) => y(t.name) + y.bandwidth() / 2);

  textGroup
    .append("text")
    .text((t) => formatFloat(t.current))
    .attr("text-anchor", "end")
    .attr("dominant-baseline", "middle")
    .style("fill", z("current"))
    .attr("x", textGroupZero + (textGroupWidth * 2) / 3)
    .attr("y", (t) => y(t.name) + y.bandwidth() / 2);

  textGroup
    .append("text")
    .text((t) => formatFloat(t.current - t.target))
    .attr("text-anchor", "end")
    .attr("dominant-baseline", "middle")
    .style("fill", (t) =>
      chroma(z1(Math.abs(t.current - t.target)))
        .darken()
        .hex()
    )
    .attr("x", textGroupZero + (textGroupWidth * 3) / 3)
    .attr("y", (t) => y(t.name) + y.bandwidth() / 2);

  textGroup
    .append("text")
    .text((t) => formatCurrency(t.current_amount ?? _.sumBy(_.values(t.aggregates ?? {}), "market_amount")))
    .attr("text-anchor", "end")
    .attr("dominant-baseline", "middle")
    .style("fill", z("current"))
    .attr("x", textGroupZero + textGroupWidth + valueColumnWidth / 2)
    .attr("y", (t) => y(t.name) + y.bandwidth() / 2);

  textGroup
    .append("text")
    .text((t) => formatCurrency(t.target_amount ?? 0))
    .attr("text-anchor", "end")
    .attr("dominant-baseline", "middle")
    .style("fill", z("target"))
    .attr("x", textGroupZero + textGroupWidth + valueColumnWidth)
    .attr("y", (t) => y(t.name) + y.bandwidth() / 2);

  const groups = g
    .append("g")
    .selectAll("g.group")
    .data(allocationTargets)
    .enter()
    .append("g")
    .attr("class", "group");

  groups
    .append("rect")
    .attr("fill", (d) => z1(Math.abs(d.target - d.current)))
    .attr("x", x1(0))
    .attr("y", (d) => y(d.name) + y.bandwidth() / 4)
    .attr("height", y.bandwidth() / 2)
    .attr("width", (d) => x1(d.current));

  groups
    .append("line")
    .attr("stroke-width", 3)
    .attr("stroke-linecap", "round")
    .attr("stroke", z("target"))
    .attr("x1", (d) => x1(d.target))
    .attr("x2", (d) => x1(d.target))
    .attr("y1", (d) => y(d.name) + y.bandwidth() / 8)
    .attr("y2", (d) => y(d.name) + (y.bandwidth() / 8) * 7);

  groups
    .append("polygon")
    .attr(
      "transform",
      (d) => "translate(" + x1(d.target) + "," + (y(d.name) + y.bandwidth() / 8) + ")"
    )
    .attr("points", "0 0, 0 15, 20 6")
    .attr("fill", z("target"));

  const paddingTop = (y1.range()[1] - y1.bandwidth() * 2) / 2;
  d3.select(`#${containerId}-treemap`)
    .append("div")
    .style("height", height + margin.top + margin.bottom + "px")
    .style("position", "absolute")
    .style("width", "100%")
    .selectAll("div")
    .data(allocationTargets)
    .enter()
    .append("div")
    .style("position", "absolute")
    .style("left", margin.left + x(0) + "px")
    .style("top", (t) => margin.top + y(t.name) + paddingTop + "px")
    .style("height", y1.bandwidth() * 2 + "px")
    .style("width", x.range()[1] - x.range()[0] + "px")
    .append("div")
    .style("position", "relative")
    .style("height", y1.bandwidth() * 2 + "px")
    .each(function (t) {
      renderPartition(this, t.aggregates, d3.treemap(), color, {
        margin: { top: 0, right: 0, bottom: 0, left: 0 }
      });
    });
}

export function renderAllocation(
  aggregates: Record<string, Aggregate>,
  color: d3.ScaleOrdinal<string, string>
) {
  renderPartition(
    document.getElementById("d3-allocation-category"),
    aggregates,
    d3.partition(),
    color
  );
  renderPartition(document.getElementById("d3-allocation-value"), aggregates, d3.treemap(), color);
}

function renderPartition(
  element: HTMLElement,
  aggregates: Record<string, Aggregate>,
  hierarchy: any,
  color: d3.ScaleOrdinal<string, string>,
  options = { margin: { top: 0, right: 20, bottom: 0, left: 0 } }
) {
  if (_.isEmpty(aggregates)) {
    return;
  }

  const div = d3.select(element),
    margin = options.margin,
    width = element.parentElement.clientWidth - margin.left - margin.right,
    height = +div.style("height").replace("px", "") - margin.top - margin.bottom;

  const percent = (d: d3.HierarchyNode<Aggregate>) => {
    return formatFloat((d.value / root.value) * 100) + "%";
  };

  const stratify = d3
    .stratify<Aggregate>()
    .id((d) => d.account)
    .parentId((d) => parentName(d.account));

  const partition = hierarchy.size([width, height]).round(true);

  const root = stratify(_.sortBy(aggregates, (a) => a.account))
    .sum((a) => a.market_amount)
    .sort(function (a, b) {
      return b.height - a.height || b.value - a.value;
    });

  partition(root);

  const cell = div
    .selectAll(".node")
    .data(root.descendants())
    .enter()
    .append("div")
    .attr("class", "node")
    .attr("data-tippy-content", (d) => {
      return tooltip([
        ["Account", [d.id, "has-text-right"]],
        ["Market Value", [formatCurrency(d.value), "has-text-weight-bold has-text-right"]],
        ["Percentage", [percent(d), "has-text-weight-bold has-text-right"]]
      ]);
    })
    .style("top", (d: any) => d.y0 + "px")
    .style("left", (d: any) => d.x0 + "px")
    .style("width", (d: any) => d.x1 - d.x0 + "px")
    .style("height", (d: any) => d.y1 - d.y0 + "px")
    .style("background", (d) => color(d.id))
    .style("color", (d) => darkenOrLighten(color(d.id)));

  cell
    .append("p")
    .attr("class", "heading has-text-weight-bold")
    .text((d) => lastName(d.id));

  cell
    .append("p")
    .attr("class", "heading has-text-weight-bold")
    .style("font-size", ".5 rem")
    .text(percent);
}

function balancesToAggregates(
  balances: Record<string, AssetBreakdown>
): { aggregates: Record<string, Aggregate>; depth: number } | null {
  const leafNodes: Aggregate[] = Object.values(balances)
    .filter((b) => b.marketAmount > 0)
    .map((b) => ({
      date: null as unknown as dayjs.Dayjs,
      account: b.group,
      market_amount: b.marketAmount,
      percent: 0
    }));

  if (leafNodes.length === 0) {
    return null;
  }

  // Build intermediate parent nodes for the hierarchy
  const allAccounts = new Set<string>();
  let maxDepth = 0;
  for (const node of leafNodes) {
    allAccounts.add(node.account);
    const parts = node.account.split(":");
    maxDepth = Math.max(maxDepth, parts.length);
    for (let i = 1; i < parts.length; i++) {
      allAccounts.add(parts.slice(0, i).join(":"));
    }
  }

  const aggregates: Record<string, Aggregate> = {};
  for (const account of allAccounts) {
    const existing = leafNodes.find((n) => n.account === account);
    aggregates[account] =
      existing || { date: null as unknown as dayjs.Dayjs, account, market_amount: 0, percent: 0 };
  }

  return { aggregates, depth: maxDepth };
}

export function renderBalanceCategoryMap(
  balances: Record<string, AssetBreakdown>,
  containerId: string
): number {
  if (_.isEmpty(balances)) {
    return 0;
  }

  const result = balancesToAggregates(balances);
  if (!result) return 0;

  const { aggregates, depth } = result;
  const accounts = Object.keys(aggregates);
  const color = generateColorScheme(accounts);
  const element = document.getElementById(containerId);
  if (!element) return 0;

  element.style.height = depth * 100 + "px";
  renderPartition(element, aggregates, d3.partition(), color);
  return depth;
}

export function renderAllocationTimeline(
  aggregatesTimeline: { [key: string]: Aggregate }[]
): Legend[] {
  const timeline = _.map(aggregatesTimeline, (aggregates) => {
    return _.chain(aggregates)
      .values()
      .filter((a) => a.market_amount != 0)
      .groupBy((a) => secondName(a.account))
      .map((aggregates, group) => {
        return {
          date: aggregates[0].date,
          account: group,
          market_amount: _.sum(_.map(aggregates, (a) => a.market_amount)),
          timestamp: aggregates[0].date
        };
      })
      .value();
  });
  const assets = _.chain(timeline)
    .last()
    .map((a) => a.account)
    .sort()
    .value();

  const defaultValues = _.zipObject(
    assets,
    _.map(assets, () => 0)
  );
  const start = timeline[0]?.[0]?.timestamp,
    end = now();

  if (!start) {
    return [];
  }

  interface Point {
    date: dayjs.Dayjs;
    [key: string]: number | dayjs.Dayjs;
  }
  const points: Point[] = [];
  _.each(timeline, (aggregates) => {
    const total = _.sum(_.map(aggregates, (a) => a.market_amount));
    if (total == 0) {
      return;
    }
    const kvs = _.map(aggregates, (a) => [a.account, (a.market_amount / total) * 100]);
    points.push(
      _.merge(
        {
          date: aggregates[0].timestamp
        },
        defaultValues,
        _.fromPairs(kvs)
      )
    );
  });

  const svg = d3.select("#d3-allocation-timeline"),
    margin = { top: 40, right: 60, bottom: 20, left: 35 },
    width =
      document.getElementById("d3-allocation-timeline").parentElement.clientWidth -
      margin.left -
      margin.right,
    height = +svg.attr("height") - margin.top - margin.bottom,
    g = svg.append("g").attr("transform", "translate(" + margin.left + "," + margin.top + ")");

  const x = d3.scaleTime().range([0, width]).domain([start, end]),
    y = d3
      .scaleLinear()
      .range([height, 0])
      .domain([0, d3.max(d3.map(points, (p) => d3.max(_.values(_.omit(p, "date")))))]),
    z = generateColorScheme(assets);

  const line = (group: string) =>
    d3
      .line<Point>()
      .curve(d3.curveLinear)
      .defined((p, i) => (p[group] as number) > 0 || (points[i + 1]?.[group] as number) > 0)
      .x((p) => x(p.date))
      .y((p) => y(p[group]));

  g.append("g")
    .attr("class", "axis x")
    .attr("transform", "translate(0," + height + ")")
    .call(d3.axisBottom(x));

  g.append("g")
    .attr("class", "axis y")
    .call(
      d3
        .axisLeft(y)
        .tickSize(-width)
        .tickFormat((y) => `${y}%`)
    );
  g.append("g")
    .attr("class", "axis y")
    .attr("transform", `translate(${width},0)`)
    .call(d3.axisRight(y).tickFormat((y) => `${y}%`));

  const layer = g.selectAll(".layer").data(assets).enter().append("g").attr("class", "layer");

  layer
    .append("path")
    .attr("fill", "none")
    .attr("stroke", (group) => z(group))
    .attr("stroke-width", "2")
    .attr("d", (group) => line(group)(points));

  return assets.map((a) => {
    return {
      label: a,
      color: z(a),
      shape: "square"
    };
  });
}

function isAccountMatched(account: string, patterns: string[]) {
  return _.some(patterns, (pattern) => {
    const regexStr = pattern
      .replace(/[.+^${}()|[\]\\]/g, "\\$&")
      .replace(/\*/g, ".*")
      .replace(/\?/g, ".")
      .replace(/%/g, ".*");
    const regex = new RegExp(`^${regexStr}$`);
    return regex.test(account) || account.startsWith(pattern + ":");
  });
}

export function computeGoalAllocationTimeline(
  postings: Posting[],
  allocationTargets: AllocationTarget[]
): { [key: string]: Aggregate }[] {
  if (_.isEmpty(postings) || _.isEmpty(allocationTargets)) {
    return [];
  }

  const sortedPostings = _.sortBy(postings, (p) => p.date.valueOf());
  const categories = _.map(allocationTargets, (t) => t.name);
  const currentBalances: Record<string, number> = _.fromPairs(categories.map((c) => [c, 0]));

  const timeline: { [key: string]: Aggregate }[] = [];
  const grouped = _.groupBy(sortedPostings, (p) => p.date.startOf("month").valueOf());
  const months = _.keys(grouped).sort();

  for (const month of months) {
    const monthPostings = grouped[month];
    for (const p of monthPostings) {
      for (const t of allocationTargets) {
        if (isAccountMatched(p.account, t.accounts)) {
          currentBalances[t.name] += p.market_amount || p.amount;
        }
      }
    }

    const monthAggregates: Record<string, Aggregate> = {};
    for (const category of categories) {
      monthAggregates[category] = {
        date: dayjs(Number(month)),
        account: category,
        market_amount: currentBalances[category],
        percent: 0
      };
    }
    timeline.push(monthAggregates);
  }

  return timeline;
}

export function renderCompositionAreaChart(
  aggregatesTimeline: { [key: string]: Aggregate }[],
  element: Element
): Legend[] {
  if (_.isEmpty(aggregatesTimeline)) {
    return [];
  }

  const firstEntry = _.first(aggregatesTimeline);
  const start = firstEntry[_.keys(firstEntry)[0]].date;
  const end = now();

  const assets = _.chain(aggregatesTimeline)
    .flatMap(_.keys)
    .uniq()
    .sort()
    .value();

  if (_.isEmpty(assets) || !element) {
    return [];
  }

  const data = _.map(aggregatesTimeline, (aggregates) => {
    const d: any = { date: aggregates[_.keys(aggregates)[0]].date };
    _.each(assets, (asset) => {
      d[asset] = Math.max(0, aggregates[asset]?.market_amount || 0);
    });
    return d;
  });

  const svg = d3.select(element);
  svg.selectAll("*").remove();

  const margin = { top: 20, right: 60, bottom: 30, left: 50 },
    width = (element.parentElement?.clientWidth || 0) - margin.left - margin.right,
    height = +svg.attr("height") - margin.top - margin.bottom,
    g = svg.append("g").attr("transform", "translate(" + margin.left + "," + margin.top + ")");

  if (width <= 0 || height <= 0) {
    return [];
  }

  svg.attr("width", width + margin.left + margin.right);

  const x = d3.scaleTime().range([0, width]).domain([start, end]),
    y = d3.scaleLinear().range([height, 0]),
    z = generateColorScheme(assets);

  const stack = d3.stack().keys(assets).offset(d3.stackOffsetExpand);
  const series = stack(data);

  const area = d3
    .area<any>()
    .x((d) => x(d.data.date))
    .y0((d) => y(d[0]))
    .y1((d) => y(d[1]));

  g.append("g")
    .attr("class", "axis x")
    .attr("transform", "translate(0," + height + ")")
    .call(d3.axisBottom(x));

  const yAxisFormat = d3.format(".0%");
  g.append("g")
    .attr("class", "axis y")
    .call(d3.axisLeft(y).tickSize(-width).tickFormat(yAxisFormat));

  g.append("g")
    .attr("class", "axis y")
    .attr("transform", `translate(${width}, 0)`)
    .call(d3.axisRight(y).tickFormat(yAxisFormat));

  const layers = g.selectAll(".layer").data(series).enter().append("g").attr("class", "layer");

  layers
    .append("path")
    .attr("class", "area")
    .style("fill", (d) => z(d.key))
    .style("opacity", 0.7)
    .attr("d", area);

  const voronoiPoints: [number, number, any][] = [];
  _.each(data, (d) => {
    const xPos = x(d.date);
    let yCumulative = 0;
    const total = _.sumBy(assets, (a) => d[a]);
    if (total === 0) return;

    _.each(assets, (asset) => {
      const val = d[asset] / total;
      yCumulative += val;
      voronoiPoints.push([xPos, y(yCumulative - val / 2), { date: d.date, asset, value: val }]);
    });
  });

  if (!_.isEmpty(voronoiPoints)) {
    const delaunay = d3.Delaunay.from(
      voronoiPoints,
      (d) => d[0],
      (d) => d[1]
    );
    const voronoi = delaunay.voronoi([0, 0, width, height]);
    const hoverCircle = g.append("circle").attr("r", "3").attr("fill", "none");
    const t = tippy(hoverCircle.node(), { theme: "light", delay: 0, allowHTML: true });

    g.append("g")
      .selectAll("path")
      .data(voronoiPoints)
      .enter()
      .append("path")
      .style("pointer-events", "all")
      .style("fill", "none")
      .attr("d", (_, i) => voronoi.renderCell(i))
      .on("mouseover", (_, d) => {
        const info = d[2];
        hoverCircle.attr("cx", d[0]).attr("cy", d[1]).attr("fill", z(info.asset));
        t.setProps({
          content: tooltip([
            ["Date", info.date.format("MMM YYYY")],
            ["Category", info.asset],
            ["Percentage", [d3.format(".2%")(info.value), "has-text-weight-bold has-text-right"]]
          ])
        });
        t.show();
      })
      .on("mouseout", () => {
        t.hide();
        hoverCircle.attr("fill", "none");
      });
  }

  return assets.map((a) => ({
    label: a,
    color: z(a),
    shape: "square"
  }));
}
