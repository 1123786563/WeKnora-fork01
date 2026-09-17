"""Create a contact sheet from the actually rendered HTML screenshots.
Uses an installed system font only; no font file is copied or distributed.
"""
from pathlib import Path
from PIL import Image, ImageDraw, ImageFont
import subprocess
R=Path(__file__).resolve().parent
try:
 font_path=subprocess.check_output(['fc-match','-f','%{file}','Noto Sans CJK SC'],text=True).strip()
except Exception:
 font_path=''
def font(size):
 try:return ImageFont.truetype(font_path,size)
 except Exception:return ImageFont.load_default()
W,H=1020,1536
im=Image.new('RGB',(W,H),'#F5F7F2');d=ImageDraw.Draw(im)
d.text((40,28),'WeKnora · Quiet Work',font=font(34),fill='#103D32')
d.text((40,82),'微信小程序高保真设计 · 6 个核心页面 · 所有业务内容为虚构示例',font=font(18),fill='#566B63')
items=[('03-home','工作台'),('06-chat','知识与 Agent 对话'),('07-tasks','任务中心'),('08-execution','执行详情'),('09-approval','审批确认'),('16-usage','用量与套餐')]
for i,(id,title) in enumerate(items):
 x=40+(i%3)*320;y=131+(i//3)*700
 d.text((x,y),id[:2]+' / '+title,font=font(18),fill='#172B27')
 pic=Image.open(R/'screenshots'/f'{id}.png').convert('RGB').resize((300,649),Image.Resampling.LANCZOS)
 im.paste(pic,(x,y+33));d.rounded_rectangle((x-1,y+32,x+300,y+682),radius=2,outline='#DBE4DD',width=1)
im.save(R/'overview.png')
print('Created overview.png from six rendered HTML pages')
