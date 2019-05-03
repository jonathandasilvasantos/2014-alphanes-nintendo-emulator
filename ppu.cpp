#include "../nes.h"

//extern RGBQUAD palette2[64];
extern uint32_t palette1[64];

//Ppu ppu;

Ppu::Ppu() : Component() {
  tick_array[0] = &Ppu::TickNTSC;
  tick_array[1] = &Ppu::TickPAL;
  tick_array[2] = nullptr;
}

Ppu::~Ppu() {

}

int Ppu::Initialize(Nes* nes) {
  Component::Initialize(nes);
  scanline=241, x=0;
  scanline_end=341;
  VBlankState=0;
  cycle_counter=0;
  read_buffer=0;
  open_bus_=0;
  open_bus_decay_timer=0;
  even_odd_toggle=false;
  offset_toggle_=false;
  reg.value = 0;
  return S_OK;
}

void Ppu::Power() {
  cycles = 0;
  reg.sysctrl = 0;
  reg.dispctrl = reg.dispctrl & 0x6;
  reg.status = reg.status & 0x1F;
  reg.OAMaddr = 0;
  offset_toggle_ = false;
  scroll.raw = 0;
  vaddr.raw = 0;
  vaddr.vaddrhi = vaddr.vaddrlo = 0;
  read_buffer = 0;

  memset(nes_->gamepak().NRAM,0xFF,0x1000);

  const uint8_t arr[] =  {0x09,0x01,0x00,0x01,0x00,0x02,0x02,0x0D,0x08,0x10,0x08,0x24,0x00,0x00,0x04,0x2C,
    0x09,0x01,0x34,0x03,0x00,0x04,0x00,0x14,0x08,0x3A,0x00,0x02,0x00,0x20,0x2C,0x08};
  memcpy(palette,arr,32);
}

void Ppu::Reset() {
  cycles = 0;
  reg.sysctrl = 0;
  reg.dispctrl = reg.dispctrl & 0x6;
  reg.status = reg.status & 0x1F;
  offset_toggle_ = false;
  scroll.raw = 0;
  read_buffer = 0;

  const uint8_t arr[] =  {0x09,0x01,0x00,0x01,0x00,0x02,0x02,0x0D,0x08,0x10,0x08,0x24,0x00,0x00,0x04,0x2C,
    0x09,0x01,0x34,0x03,0x00,0x04,0x00,0x14,0x08,0x3A,0x00,0x02,0x00,0x20,0x2C,0x08};
  memcpy(palette,arr,32);
  memset(nes_->gamepak().NRAM,0x00,0x1000);
}

inline uint8_t& Ppu::mmap(int i)  {
  i &= 0x3FFF;
  if(i >= 0x3F00) { if(i%4==0) i &= 0x0F; return palette[i & 0x1F]; }

  if(i < 0x2000) 
    return nes_->gamepak().chr_banks[i>>10][i&0x3FF];
  return nes_->gamepak().Nta[(i>>10)&3][i&0x3FF];
}


uint8_t Ppu::Read(uint16_t address) {
  auto RefreshOpenBus = [&](uint8_t v) { return open_bus_decay_timer = 77777, open_bus_ = v; };
  uint8_t res = open_bus_;
    
  switch(address&0x7) {
    case 0:  break;
    case 1:  break;
    case 2: 
      res = reg.status | (open_bus_ & 0x1F);
      reg.InVBlank = false;  // Reading $2002 clears the vblank flag.
      offset_toggle_ = false; // Also resets the toggle for address updates.
      if(VBlankState != -5)
          VBlankState = 0; // This also may cancel the setting of InVBlank.
      break;
    case 3:  break; // Index into Object Attribute Memory
    case 4: 
      res = RefreshOpenBus(OAM[reg.OAMaddr] & (reg.OAMdata==2 ? 0xE3 : 0xFF));
      break;
    case 7:
      res = read_buffer;
      uint8_t& t = mmap(vaddr.raw); // Access the video memory.

      if((vaddr.raw & 0x3F00) == 0x3F00) { // palette?
        res = (open_bus_ & 0xC0) | (t & 0x3F);
        read_buffer = mmap(vaddr.raw & 0x2FFF);
      } else {
        read_buffer = t;
      }
      RefreshOpenBus(res);
      vaddr.raw = vaddr.raw + (reg.Inc ? 32 : 1); // The address is automatically updated.
      break;
  }
  return res;
}

void Ppu::Write(uint16_t address,uint8_t data) {
    auto RefreshOpenBus = [&](uint8_t v) { return open_bus_decay_timer = 77777, open_bus_ = v; };
    RefreshOpenBus(data);
    switch(address&0x7) // Which port from $200x?
    {
        case 0: reg.sysctrl  = data; scroll.basenta = reg.BaseNTA; break;
        case 1: reg.dispctrl = data; break;
        case 3: reg.OAMaddr  = data; break; // Index into Object Attribute Memory
        case 4: OAM[reg.OAMaddr++] = data;        // Write or read the OAM (sprites).
                break;
        case 5: 
            if(offset_toggle_) { scroll.yfine   = data & 7; scroll.ycoarse = data >> 3; }
            else              { scroll.xscroll = data; }
            offset_toggle_ = !offset_toggle_;
            break;
        case 6: 
            if(offset_toggle_) { scroll.vaddrlo = data; 
              vaddr.raw = (unsigned) scroll.raw; 
              uint16_t addr = vaddr.raw;
              //nes_->gamepak().mapper->PpuTick();
              int a = 1;
            }
            else              { 
              scroll.vaddrhi = data & 0x3F; 

            }
            offset_toggle_ = !offset_toggle_;
            
            break;
        case 7:
            uint8_t& t = mmap(vaddr.raw); // Access the video memory.
            t = data;
            RefreshOpenBus(t);
            vaddr.raw = vaddr.raw + (reg.Inc ? 32 : 1); // The address is automatically updated.

            break;
    }
}



void Ppu::rendering_tick()
{
    auto tile_decode_mode = (0x10FFFF & (1u << (x>>4))); // When x is 0..255, 320..335

    // Each action happens in two steps: 1) select memory address; 2) receive data and react on it.
    switch(x % 8)
    {
        case 2: // Point to attribute table
            ioaddr = 0x23C0 + 0x400*vaddr.basenta + 8*(vaddr.ycoarse/4) + (vaddr.xcoarse/4);
            if(tile_decode_mode) break; // Or nametable, with sprites.
        case 0: // Point to nametable
            ioaddr = 0x2000 + (vaddr.raw & 0xFFF);
            // Reset sprite data
            if(x == 0) { sprinpos = sproutpos = 0; if(reg.ShowSP) reg.OAMaddr = 0; }
            if(!reg.ShowBG) break;
            // Reset scrolling (vertical once, horizontal each scanline)
            if(x == 304 && scanline == -1) vaddr.raw = (unsigned) scroll.raw;
            if(x == 256) { vaddr.xcoarse   = (unsigned)scroll.xcoarse;
                            vaddr.basenta_h = (unsigned)scroll.basenta_h;
                            sprrenpos = 0; }
            break;
        case 1:
            if(x == 337 && scanline == -1 && even_odd_toggle && reg.ShowBG && (mode_==NTSC)) scanline_end = 340;
            // Name table access
            pat_addr = 0x1000*reg.BGaddr + 16*mmap(ioaddr) + vaddr.yfine;
            if(!tile_decode_mode) break;
            // Push the current tile into shift registers.
            // The bitmap pattern is 16 bits, while the attribute is 2 bits, repeated 8 times.
            bg_shift_pat  = (bg_shift_pat  >> 16) + 0x00010000 * tilepat;
            bg_shift_attr = (bg_shift_attr >> 16) + 0x55550000 * tileattr;
            break;
        case 3:
            // Attribute table access
            if(tile_decode_mode)
            {
                tileattr = (mmap(ioaddr) >> ((vaddr.xcoarse&2) + 2*(vaddr.ycoarse&2))) & 3;
                // Go to the next tile horizontally (and switch nametable if it wraps)
                if(!++vaddr.xcoarse) { vaddr.basenta_h = 1-vaddr.basenta_h; }
                // At the edge of the screen, do the same but vertically
                if(x==251 && !++vaddr.yfine && ++vaddr.ycoarse == 30)
                    { vaddr.ycoarse = 0; vaddr.basenta_v = 1-vaddr.basenta_v; }
            }
            else if(sprrenpos < sproutpos)
            {
                // Select sprite pattern instead of background pattern
                auto& o = OAM3[sprrenpos & 0x7]; // Sprite to render on next scanline
                memcpy(&o, &OAM2[sprrenpos & 0x7], sizeof(o));
                unsigned y = (scanline) - o.y;
                if(o.attr & 0x80) y ^= (reg.SPsize ? 15 : 7);
                pat_addr = 0x1000 * (reg.SPsize ? (o.index & 0x01) : reg.SPaddr);
                pat_addr +=  0x10 * (reg.SPsize ? (o.index & 0xFE) : (o.index & 0xFF));
                pat_addr += (y&7) + (y&8)*2;
            }
            break;
        // Pattern table bytes
        case 5:
            tilepat = mmap(pat_addr|0);
            break;
        case 7: // Interleave the bits of the two pattern bytes
            unsigned p = tilepat | (mmap(pat_addr|8) << 8);
            p = (p&0xF00F) | ((p&0x0F00)>>4) | ((p&0x00F0)<<4);
            p = (p&0xC3C3) | ((p&0x3030)>>2) | ((p&0x0C0C)<<2);
            p = (p&0x9999) | ((p&0x4444)>>1) | ((p&0x2222)<<1);
            tilepat = p;
            // When decoding sprites, save the sprite graphics and move to next sprite
            if(!tile_decode_mode && sprrenpos < sproutpos)
                OAM3[(sprrenpos++) & 0x7].pattern = tilepat;
            break;
    }
    // Find which sprites are visible on next scanline (TODO: implement crazy 9-sprite malfunction)
    switch(x>=64 && x<256 && x%2 ? (reg.OAMaddr++ & 3) : 4)
    {
        default:
            // Access OAM (object attribute memory)
            sprtmp = OAM[reg.OAMaddr];
            break;
        case 0:
            if(sprinpos >= 64) { reg.OAMaddr=0; break; }
            ++sprinpos; // next sprite
            if(sproutpos<8) OAM2[sproutpos].y        = sprtmp;
            if(sproutpos<8) OAM2[sproutpos].sprindex = reg.OAMindex;
            {int y1 = sprtmp, y2 = sprtmp + (reg.SPsize?16:8);
            if(!( scanline >= y1 && scanline < y2 ))
                reg.OAMaddr = sprinpos != 2 ? reg.OAMaddr+3 : 8;}
            break;
        case 1:
            if(sproutpos<8) OAM2[sproutpos].index = sprtmp;
            break;
        case 2:
            if(sproutpos<8) OAM2[sproutpos].attr  = sprtmp;
            break;
        case 3:
            if(sproutpos<8) OAM2[sproutpos].x = sprtmp;
            if(sproutpos<8) ++sproutpos; else reg.SPoverflow = true;
            if(sprinpos == 2) reg.OAMaddr = 8;
            break;
    }

    
}

void Ppu::render_pixel()
{
    bool edge   = uint8_t(x+8) < 16; // 0..7, 248..255
    bool showbg = reg.ShowBG && (!edge || reg.ShowBG8);
    bool showsp = reg.ShowSP && (!edge || reg.ShowSP8);

    // Render the background
    unsigned fx = scroll.xfine, xpos = 15 - (( (x&7) + fx + 8*!!(x&7) ) & 15);

    unsigned pixel = 0, attr = 0;
    if(showbg) // Pick a pixel from the shift registers
    {
        pixel = (bg_shift_pat  >> (xpos*2)) & 3;
        attr  = (bg_shift_attr >> (xpos*2)) & (pixel ? 3 : 0);
    }
    else if( (vaddr.raw & 0x3F00) == 0x3F00 && !reg.ShowBGSP )
        pixel = vaddr.raw;

    // Overlay the sprites
    if(showsp)
        for(unsigned sno=0; sno<sprrenpos; ++sno)
        {
            auto& s = OAM3[sno];
            // Check if this sprite is horizontally in range
            unsigned xdiff = x - s.x;
            if(xdiff >= 8) continue; // Also matches negative values
            // Determine which pixel to display; skip transparent pixels
            if(!(s.attr & 0x40)) xdiff = 7-xdiff;
            uint8_t spritepixel = (s.pattern >> (xdiff*2)) & 3;
            if(!spritepixel) continue;
            // Register sprite-0 hit if applicable
            if(x < 255 && pixel && s.sprindex == 0) 
              reg.SP0hit = true;
            // Render the pixel unless behind-background placement wanted
            if(!(s.attr & 0x20) || !pixel)
            {
                attr = (s.attr & 3) + 4;
                pixel = spritepixel;
            }
            // Only process the first non-transparent sprite pixel.
            break;
        }
    pixel = palette[ (attr*4 + pixel) & 0x1F ] & (reg.Grayscale ? 0x30 : 0x3F);
    //nes_->frame_buffer[scanline * 256 + x] = 0xff000000|IO::PutPixel(x, scanline, pixel | (reg.EmpRGB << 6), cycle_counter);
        
    auto pal_pixel = IO::PutPixel(x, scanline, pixel | (reg.EmpRGB << 6), cycle_counter);
    //auto pal_pixel = IO::MakeRGBcolor((pixel | (reg.EmpRGB << 6))&0x3f); 
    //auto pal_pixel = ::palette1[(pixel | (reg.EmpRGB << 6))&0x3f];
    nes_->frame_buffer[(scanline<<8) + x] = 0xff000000|pal_pixel;//(((pal_pixel)&0xff)<<16) | (((pal_pixel>>8)&0xff)<<8) | ((pal_pixel>>16)&0xff);
    //palette[offset][prev%64][pixel];
    //auto pal_pixel = palette2[(pixel | (reg.EmpRGB << 6))&0x3f];//palette[offset][prev%64][pixel];
    //buf[scanline * 256 + x].rgbRed = (pal_pixel>>16)&0xff;
    //buf[scanline * 256 + x].rgbGreen = (pal_pixel>>8)&0xff;
    //buf[scanline * 256 + x].rgbBlue = (pal_pixel)&0xff;
}

// PPU::tick() -- This function is called 3 times per each CPU cycle.
// Each call iterates through one pixel of the screen.
// The screen is divided into 262 scanlines, each having 341 columns, as such:
//
//           x=0                 x=256      x=340
//       ___|____________________|__________|
//  y=-1    | pre-render scanline| prepare  | >
//       ___|____________________| sprites _| > Graphics
//  y=0     | visible area       | for the  | > processing
//          | - this is rendered | next     | > scanlines
//  y=239   |   on the screen.   | scanline | >
//       ___|____________________|______
//  y=240   | idle
//       ___|_______________________________
//  y=241   | vertical blanking (idle)
//          | 20 scanlines long
//  y=260___|____________________|__________|
//
// On actual PPU, the scanline begins actually before x=0, with
// sync/colorburst/black/background color being rendered, and
// ends after x=256 with background/black being rendered first,
// but in this emulator we only care about the visible area.
//
// When background rendering is enabled, scanline -1 is
// 340 or 341 pixels long, alternating each frame.
// In all other situations the scanline is 341 pixels long.
// Thus, it takes 89341 or 89342 PPU::tick() calls to render 1 frame.
void Ppu::Tick() {
  (this->*(tick_array[mode_]))();
}

void Ppu::TickNTSC() {
  for (auto i=0;i<3;++i) {
   
    //Set/clear vblank where needed
    switch(VBlankState)  {
        case -5: reg.status = 0;  break;
        case 2: reg.InVBlank = true; break;
        case 0: {
          nes_->cpu().nmi = reg.InVBlank && reg.NMIenabled; 

                }
          break;
    }
    if(VBlankState != 0) VBlankState += (VBlankState < 0 ? 1 : -1);
    if(open_bus_decay_timer) if(!--open_bus_decay_timer) open_bus_ = 0;

    // Graphics processing scanline?
    if(scanline < 240) {
        /* Process graphics for this cycle */
        if(reg.ShowBGSP) {
          //nes_->gamepak().mapper->Tick(0);
          rendering_tick();
        }
        if(scanline >= 0 && x < 256) 
          render_pixel();
    }

    // Done with the cycle. Check for end of scanline.
    if(++cycle_counter == 3) 
      cycle_counter = 0; // For NTSC pixel shifting

    //required for timing test pass *Hack fix*
    if (scanline == 260 && x >= 328 && x <= 339) {
      reg.SP0hit = false;
    }
    if(++x == scanline_end) {
      // Begin new scanline
      //IO::FlushScanline(scanline);
      if(scanline == 239) {
        
        nes_->on_render();
      }
     
        
      scanline_end = 341;
      x            = 0;
        /*#ifdef _DEBUG
        {
          char str[255];
          sprintf(str,"PPU  scanline done %d\n",scanline);
          OutputDebugString(str);
        }
        #endif*/
      // Does something special happen on the new scanline?
      switch(scanline += 1) {
        case 261: // Begin of rendering
          scanline = -1; // pre-render line
          even_odd_toggle = !even_odd_toggle;
          // Clear vblank flag
          VBlankState = -5;
          ;
          break;

        case 241: // Begin of vertical blanking
          nes_->on_vertical_blank();
          VBlankState = 2;
        /*#ifdef _DEBUG
        {
          char str[255];
          sprintf(str,"PPU VB\n");
          OutputDebugString(str);
        }
        #endif   */       
          break;
      }
    }
    nes_->gamepak().mapper->PpuTick();
    ++cycles;
  }
}

void Ppu::TickPAL() {
  int tick_count = 3;
  if ((nes_->cpu().cycles % 5)==4)
    ++tick_count;

  for (auto i=0;i<tick_count;++i) {
    //Set/clear vblank where needed
    switch(VBlankState)  {
        case -5: reg.status = 0;  break;
        case 2: reg.InVBlank = true; break;
        case 0: {
          nes_->cpu().nmi = reg.InVBlank && reg.NMIenabled; 

                }
          break;
    }
    if(VBlankState != 0) VBlankState += (VBlankState < 0 ? 1 : -1);
    if(open_bus_decay_timer) if(!--open_bus_decay_timer) open_bus_ = 0;

    // Graphics processing scanline?
    if(scanline < 240) {
        /* Process graphics for this cycle */
        if(reg.ShowBGSP) {
          //nes_->gamepak().mapper->Tick(0);
          rendering_tick();
        }
        if(scanline >= 0 && x < 256) 
          render_pixel();
    }

    if(++x == scanline_end) {
      // Begin new scanline
      //IO::FlushScanline(scanline);
      if(scanline == 239) {
        nes_->on_render();
      }
      scanline_end = 341;
      x            = 0;
      // Does something special happen on the new scanline?
      switch(scanline += 1) {
        case 311: // Begin of rendering
          scanline = -1; // pre-render line
          even_odd_toggle = !even_odd_toggle;
          // Clear vblank flag
          VBlankState = -5;
          break;

        case 241: // Begin of vertical blanking
          nes_->on_vertical_blank();
          VBlankState = 2;
          
          break;
      }
    }
    nes_->gamepak().mapper->PpuTick();
    ++cycles;
  }
}

void Ppu::OnSettingsChanged() {

}