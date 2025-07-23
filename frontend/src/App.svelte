<script>
  import { onMount, onDestroy, tick } from 'svelte';
  import * as d3 from 'd3';
  import { Tabs, TabItem, Button, Select } from 'flowbite-svelte';

  let datasets = {};
  let groups = [];
  let allGraphs = [];
  let refreshInterval = 0; // seconds; 0 disables auto-refresh
  let timer;
  let activeTab = 0;
  const refreshOptions = [
    { value: 0, name: 'Off' },
    { value: 5, name: '5s' },
    { value: 10, name: '10s' },
    { value: 30, name: '30s' },
    { value: 60, name: '60s' }
  ];

  async function fetchGroups() {
    const res = await fetch('/api/graphs', {headers: {'Authorization': localStorage.getItem('token') || ''}});
    const data = await res.json();
    groups = data.groups || [];
    allGraphs = data.graphs || [];
    if (groups.length === 0) {
      groups = [{ name: 'All', graphs: allGraphs }];
    } else {
      const grouped = new Set(groups.flatMap(g => g.graphs));
      const ungrouped = allGraphs.filter(g => !grouped.has(g));
      if (ungrouped.length) {
        groups = [...groups, { name: 'Other', graphs: ungrouped }];
      }
    }
  }

  async function fetchData() {
    const list = groups[activeTab]?.graphs || allGraphs;
    if (!list || list.length === 0) {
      datasets = {};
      return;
    }
    const res = await fetch('/api/data?graphs=' + encodeURIComponent(list.join(',')), {headers: {'Authorization': localStorage.getItem('token') || ''}});
    const data = await res.json();
    datasets = data.graphs ? data.graphs : data;
    await tick();
    drawAll();
  }

  function safeId(name) {
    return name.replace(/[^A-Za-z0-9_-]/g, '_');
  }

  function updateTimer() {
    if (timer) {
      clearInterval(timer);
      timer = null;
    }
    if (refreshInterval > 0) {
      timer = setInterval(fetchData, refreshInterval * 1000);
    }
  }

  $: refreshInterval, updateTimer();

  function drawAll() {
    const margin = {left: 40, right: 20, top: 0, bottom: 20};
    const width = 600 - margin.left - margin.right;
    const height = 300 - margin.top - margin.bottom;
    const scale = 0.01; // sensor values are 100x larger than the displayed units
    const defaultColor = d3.scaleOrdinal(d3.schemeCategory10);

    for (const [name, info] of Object.entries(datasets)) {
      let data = info.data || [];
      const cumulative = info.cumulative || {};
      if (Object.keys(cumulative).length) {
        const grouped = d3.group(data, d => d.source);
        const transformed = [];
        for (const [src, values] of grouped) {
          if (cumulative[src]) {
            values.sort((a, b) => a.timestamp - b.timestamp);
            for (let i = 1; i < values.length; i++) {
              transformed.push({ ...values[i], value: values[i].value - values[i-1].value });
            }
          } else {
            transformed.push(...values);
          }
        }
        data = transformed;
      }
      const units = info.units || '';
      const type = info.type || '';
      let container = d3.select(`#chart-${safeId(name)}`);
      let svg = container.select('svg');
      let legendDiv = d3.select(`#legend-${safeId(name)}`);
      let g;
      if (svg.empty()) {
        svg = container.append('svg').attr('width', 600).attr('height', 300);
        g = svg.append('g').attr('transform', `translate(${margin.left},${margin.top})`);
      } else {
        g = svg.select('g');
      }
      g.selectAll('*').remove();
      if (!data.length) continue;
      data.forEach(d => { d.date = new Date(d.timestamp * 1000); });
      const x = d3.scaleTime().range([0, width]);
      x.domain(d3.extent(data, d => d.date));
      const y = d3.scaleLinear().range([height, 0]);
      const minVal = d3.min(data, d => d.value * scale);
      const maxVal = d3.max(data, d => d.value * scale);
      const padding = (maxVal - minVal) * 0.1;
      y.domain([minVal - padding, maxVal + padding]);
      const line = d3.line()
        .x(d => x(d.date))
        .y(d => y(d.value * scale));

      const sources = Array.from(new Set(data.map(d => d.source)));
      const colorFor = src => {
        if (info.colors && info.colors[src]) return info.colors[src];
        return sources.length > 1 ? defaultColor(src) : 'steelblue';
      };
      if (sources.length > 1) {
        const groups = d3.group(data, d => d.source);
        for (const [src, values] of groups) {
          let c = colorFor(src);
          g.append('path').datum(values).attr('fill', 'none').attr('stroke', c).attr('d', line);
        }
      } else {
        const src = sources[0];
        let c = colorFor(src);
        g.append('path').datum(data).attr('fill', 'none').attr('stroke', c).attr('d', line);
      }

      g.append('g').attr('transform', `translate(0,${height})`).call(d3.axisBottom(x));
      g.append('g').call(d3.axisLeft(y).tickFormat(d3.format('.2f')));
      let label = name;
      const details = [];
      if (type) details.push(type);
      if (units) details.push(units);
      if (details.length) label += ` (${details.join(' ')})`;
      g.append('text')
        .attr('x', width / 2)
        .attr('y', height + margin.bottom + 15)
        .attr('text-anchor', 'middle')
        .text(label);

      // update legend
      legendDiv.selectAll('*').remove();
      const items = legendDiv.selectAll('.legend-item')
        .data(sources)
        .enter()
        .append('div')
        .attr('class', 'legend-item')
        .style('color', d => colorFor(d));
      items.append('span')
        .attr('class', 'legend-color')
        .style('background-color', d => colorFor(d));
      items.append('span').text(d => d);
    }

  }

  onMount(async () => {
    await fetchGroups();
    fetchData();
  });
  onDestroy(() => {
    if (timer) clearInterval(timer);
  });
</script>

<Tabs style="underline">
  {#each groups as grp, i}
    <TabItem
      title={grp.name}
      open={i === activeTab}
      on:click={() => {
        activeTab = i;
        fetchData();
      }}
    />
  {/each}
</Tabs>
<div id="dashboard">
  {#if groups[activeTab]}
      {#each groups[activeTab].graphs as name}
        <div class="chart-container">
          <h3>{name}</h3>
          <div id={"chart-" + safeId(name)}></div>
          <div class="legend" id={"legend-" + safeId(name)}></div>
        </div>
      {/each}
    {/if}
  </div>
<div id="controls" class="flex items-center space-x-4 mt-4">
  <Button on:click={fetchData}>Refresh</Button>
  <label class="flex items-center space-x-2">
    <span>Auto refresh:</span>
    <Select class="w-28" items={refreshOptions} bind:value={refreshInterval} />
  </label>
</div>

<style>
  #dashboard {
    display: flex;
    flex-wrap: wrap;
  }
  .chart-container {
    margin-right: 20px;
    margin-bottom: 20px;
  }
  .legend {
    display: flex;
    flex-direction: column;
    gap: 4px;
    margin-top: 4px;
  }
  .legend-item {
    display: flex;
    align-items: center;
    font-size: 0.4375rem;
  }
  .legend-color {
    width: 12px;
    height: 12px;
    margin-right: 4px;
  }
</style>
