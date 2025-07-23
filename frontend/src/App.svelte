<script>
  import { onMount, onDestroy, tick } from 'svelte';
  import * as d3 from 'd3';

  let datasets = {};
  let refreshInterval = 0; // seconds; 0 disables auto-refresh
  let timer;

  async function fetchData() {
    const res = await fetch('/api/data', {headers: {'Authorization': localStorage.getItem('token') || ''}});
    datasets = await res.json();
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
      if (sources.length > 1) {
        const groups = d3.group(data, d => d.source);
        for (const [src, values] of groups) {
          let c = (info.colors && info.colors[src]) ? info.colors[src] : defaultColor(src);
          g.append('path').datum(values).attr('fill', 'none').attr('stroke', c).attr('d', line);
        }
      } else {
        const src = sources[0];
        let c = (info.colors && info.colors[src]) ? info.colors[src] : 'steelblue';
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
    }

  }

  onMount(fetchData);
  onDestroy(() => {
    if (timer) clearInterval(timer);
  });
</script>

<div id="dashboard">
  {#each Object.keys(datasets) as name}
    <div class="chart-container">
      <h3>{name}</h3>
      <div id={"chart-" + safeId(name)}></div>
    </div>
  {/each}
</div>
<div id="controls">
  <button on:click={fetchData}>Refresh</button>
  <label>
    Auto refresh:
    <select bind:value={refreshInterval}>
      <option value="0">Off</option>
      <option value="5">5s</option>
      <option value="10">10s</option>
      <option value="30">30s</option>
      <option value="60">60s</option>
    </select>
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
  #controls {
    margin-top: 10px;
  }
  #controls label {
    margin-left: 10px;
  }
</style>
