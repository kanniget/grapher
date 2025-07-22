<script>
  import { onMount } from 'svelte';
  import * as d3 from 'd3';

  let datasets = {};
  let selected = '';

  async function fetchData() {
    const res = await fetch('/api/data', {headers: {'Authorization': localStorage.getItem('token') || ''}});
    datasets = await res.json();
    if (!selected && Object.keys(datasets).length) {
      selected = Object.keys(datasets)[0];
    }
    draw();
  }

  let svg;
  let g;
  function draw() {
    const margin = {left: 40, right: 20, top: 0, bottom: 20};
    const width = 600 - margin.left - margin.right;
    const height = 300 - margin.top - margin.bottom;
    if (!svg) {
      svg = d3.select('#chart').append('svg').attr('width', 600).attr('height', 300);
      g = svg.append('g').attr('transform', `translate(${margin.left},${margin.top})`);
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
      .attr('transform', 'rotate(-90)')
      .attr('y', -margin.left + 10)
      .attr('x', -height / 2)
      .attr('text-anchor', 'middle')
      .text(yLabel);
  }

  onMount(fetchData);
</script>

<div id="chart"></div>

<select bind:value={selected} on:change={draw}>
  {#each Object.keys(datasets) as name}
    <option value={name}>{name}</option>
  {/each}
</select>
<button on:click={fetchData}>Refresh</button>
