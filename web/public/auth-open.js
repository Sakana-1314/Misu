const params = new URLSearchParams(window.location.search)
const target = params.get('url')

if (target && target.startsWith('https://mp.weixin.qq.com/')) {
  window.location.replace(target)
} else {
  document.body.textContent = '无效的授权链接'
}
