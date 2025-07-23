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

    for (const [name, info] of Object.entries(datasets)) {
      const data = info.data || [];
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
      const x = d3.scaleTime().range([0, width]);
      const y = d3.scaleLinear().range([height, 0]);
      data.forEach(d => { d.date = new Date(d.timestamp * 1000); });
      x.domain(d3.extent(data, d => d.date));
      const minVal = d3.min(data, d => d.value * scale);
      const maxVal = d3.max(data, d => d.value * scale);
      const padding = (maxVal - minVal) * 0.1;
      y.domain([minVal - padding, maxVal + padding]);
      const line = d3.line()
        .x(d => x(d.date))
        .y(d => y(d.value * scale));
      g.append('path').datum(data).attr('fill', 'none').attr('stroke', 'steelblue').attr('d', line);
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


    g.selectAll('*').remove();
    const ds = datasets[selected] || {};
    const data = ds.data || ds;
    if (!data || !data.length) return;
    const x = d3.scaleTime().range([0, width]);
    const y = d3.scaleLinear().range([height, 0]);
    data.forEach(d => { d.date = new Date(d.timestamp * 1000); });
    x.domain(d3.extent(data, d => d.date));
    y.domain([0, d3.max(data, d => d.value)]);
    const line = d3.line()
      .x(d => x(d.date))
      .y(d => y(d.value));
    g.append('path').datum(data).attr('fill', 'none').attr('stroke', 'steelblue').attr('d', line);
    g.append('g').attr('transform', `translate(0,${height})`).call(d3.axisBottom(x));
    g.append('g').call(d3.axisLeft(y));
    const labelParts = [];
    if (ds.type) labelParts.push(ds.type);
    if (ds.units) labelParts.push(`(${ds.units})`);
    const yLabel = labelParts.join(' ');

    g.append('text')
      .attr('x', width / 2)
      .attr('y', height + margin.bottom + 15)
      .attr('text-anchor', 'middle')
      .text(yLabel);

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
