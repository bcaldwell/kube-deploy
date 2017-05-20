#!/usr/bin/env ruby
require 'io/console'

class Printer
  ####################
  #
  #   Logging
  #
  ####################

  class << self
    def put_header(text, color = "\x1b[36m")
      put_edge(color, "┏━━ ", text)
    end

    def put_edge(color, prefix, text)
      ptext = "#{color}#{prefix}#{text}"
      textwidth = printing_width(ptext)

      console = IO.console
      termwidth = console ? console.winsize[1] : 80
      termwidth = 30 if termwidth < 30

      if textwidth > termwidth
        ptext = ptext[0...termwidth]
        textwidth = termwidth
      end
      padwidth = termwidth - textwidth
      pad = "━" * padwidth
      formatted = "#{ptext}#{color}#{pad}\x1b[0m\n"

      puts formatted
    end

    def log(msg)
      puts "\x1b[36m┃\x1b[0m " + msg
    end

    # ANSI escape sequences (like \x1b[31m) have zero width.
    # when calculating the padding width, we must exclude them.
    def printing_width(str)
      str.gsub(/\x1b\[[\d;]+[A-z]/, '').size
    end

    def put_footer(success = true, success_color = "\x1b[36m")
      if success
        put_edge(success_color, "┗", "")
      else
        text = "💥  Failed! Aborting! "
        put_edge("\x1b[31m", "┗━━ ", text)
        exit
      end
    end

    def prompt_user_with_options(question, options)
      require 'readline'

      log(question)
      log("Your options are:")
      options.each_with_index do |v, idx|
        log("#{idx + 1}) #{v}")
      end
      log("Choose a number between 1 and #{options.length}")

      Readline.completion_append_character = " "
      Readline.completion_proc = nil

      buf = -1
      available = (1..options.length).to_a
      until available.include?(buf.to_i)
        begin
          buf = Readline.readline("\x1b[34m┃ > \x1b[33m", true)
        rescue Interrupt
          nil
        end

        if buf.nil?
          STDERR.puts
          next
        end

        buf = buf.chomp
        buf = -1 if buf.empty?
        buf = -1 if buf.to_i.to_s != buf
      end

      options[buf.to_i - 1]
    end

    def puts_coloured(a, error: false)
      text = a
        .gsub(/{{green:(.*?)}}/, "\x1b[32m\\1\x1b[0m")
        .gsub(/{{bold:(.*?)}}/,  "\x1b[1m\\1\x1b[0m")
        .gsub(/{{cyan:(.*?)}}/,  "\x1b[36m\\1\x1b[0m")
        .gsub(/{{red:(.*?)}}/, "\x1b[31m\\1\x1b[0m")

      if error
        puts_red text
      else
        puts text
      end
    end

    def puts_info(a, mark = '?')
      puts_blue("\x1b[34m#{mark} \x1b[0m" + a)
    end

    def puts_success(a)
      puts_green("\x1b[32m✓\x1b[0m " + a)
    end

    def puts_failure(a)
      puts_red("\x1b[31m✗\x1b[0m " + a)
    end

    def puts_blue(a)
      puts_raw("\x1b[34m┃\x1b[0m " + a)
    end

    def puts_green(a)
      puts_raw("\x1b[32m┃\x1b[0m " + a)
    end

    def puts_red(a)
      puts_raw("\x1b[31m┃\x1b[0m " + a)
    end

    def puts_raw(a)
      STDOUT.puts(a)
    end
  end
end
